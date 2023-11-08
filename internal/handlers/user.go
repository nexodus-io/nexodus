package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// key for username in gin.Context
const AuthUserName string = "_nexodus.UserName"

// CacheExp Zero expiration means the key has no expiration time.
const CacheExp time.Duration = 0
const CachePrefix = "user:"

func (api *API) UserIsCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)

	// this could potentially be driven by rego output
	return db.Where("id = ?", userId)
}

func (api *API) CreateUserIfNotExists(ctx context.Context, idpId string, idpUsername string) (uuid.UUID, error) {
	ctx, span := tracer.Start(ctx, "createUserIfNotExists")
	defer span.End()
	span.SetAttributes(
		attribute.String("ipd_id", idpId),
		attribute.String("username", idpUsername),
	)
	var user models.User
	var userID uuid.UUID

	// Retry the operation if we get a duplicate key error which can occur on concurrent requests when creating a user
	err := util.RetryOperationForErrors(ctx, time.Millisecond*10, 1, []error{gorm.ErrDuplicatedKey}, func() error {
		return api.transaction(ctx, func(tx *gorm.DB) error {

			// First lets check if the user has ever existed in the database
			res := tx.Unscoped().First(&user, "idp_id = ?", idpId)

			// If the user exists, then lets restore their status in the database
			if res.Error == nil {

				fieldsToUpdate := []interface{}{}

				// Check if the UserName has changed since the last time we saw this user
				if user.UserName != idpUsername {
					user.UserName = idpUsername
					fieldsToUpdate = append(fieldsToUpdate, "UserName")
				}

				// do we need to undelete it?
				if user.DeletedAt.Valid {
					user.DeletedAt = gorm.DeletedAt{}
					fieldsToUpdate = append(fieldsToUpdate, "DeletedAt")
				}

				if len(fieldsToUpdate) > 0 {
					if res := tx.Unscoped().
						Model(&user).
						Select(fieldsToUpdate[0], fieldsToUpdate[1:]...).
						Updates(&user); res.Error != nil {
						return res.Error
					}
				}

				userID = user.ID
				err := api.createUserOrgIfNotExists(ctx, tx, userID, idpUsername)
				if err != nil {
					return err
				}

				return nil
			}

			if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("find failed record for idp_id %s: %w", idpId, res.Error)
			}
			userID = uuid.New()
			user.ID = userID
			user.IdpID = idpId
			user.UserName = idpUsername
			if res = tx.Create(&user); res.Error != nil {
				if database.IsDuplicateError(res.Error) {
					res.Error = gorm.ErrDuplicatedKey
				}
				return res.Error
			}
			err := api.createUserOrgIfNotExists(ctx, tx, userID, idpUsername)
			if err != nil {
				return err
			}

			return nil
		})
	})

	if err != nil {
		return uuid.Nil, fmt.Errorf("can't create user record: %w", err)
	}

	if userID != uuid.Nil {
		return userID, nil
	}

	return uuid.Nil, fmt.Errorf("can't create user record")
}

func (api *API) createUserOrgIfNotExists(ctx context.Context, tx *gorm.DB, userId uuid.UUID, userName string) error {

	// Get the first org the use owns.
	org := models.Organization{}
	res := api.db.Where("owner_id = ?", userId.String()).First(&org)
	if res.Error == nil {
		return nil
	}

	if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return res.Error
	}

	org = models.Organization{
		Base: models.Base{
			ID: userId,
		},
		OwnerID:     userId,
		Name:        userName,
		Description: fmt.Sprintf("%s's organization", userName),
		Users: []*models.User{{
			Base: models.Base{
				ID: userId,
			},
		}},
		VPCs: []*models.VPC{{
			Base: models.Base{
				ID: userId,
			},
			OrganizationID: userId,
			Description:    "default vpc",
			PrivateCidr:    false,
			Ipv4Cidr:       defaultIPAMv4Cidr,
			Ipv6Cidr:       defaultIPAMv6Cidr,
		}},
	}

	// Create the organization
	if res := tx.Create(&org); res.Error != nil {
		if database.IsDuplicateError(res.Error) {
			return res.Error
		}

		// If we already have an existing organisation then lets just use that one
		if tx.Where("owner_id = ?", userId).First(&org).Error == nil {
			return nil
		}

		return fmt.Errorf("can't create organization record: %w", res.Error)
	}

	ipamNamespace := defaultIPAMNamespace

	// Create namespaces and prefixes
	if err := api.ipam.CreateNamespace(ctx, ipamNamespace); err != nil {
		return fmt.Errorf("failed to create ipam namespace: %w", err)
	}
	if err := api.ipam.AssignCIDR(ctx, ipamNamespace, defaultIPAMv4Cidr); err != nil {
		return fmt.Errorf("can't assign default ipam v4 prefix: %w", err)
	}
	if err := api.ipam.AssignCIDR(ctx, ipamNamespace, defaultIPAMv6Cidr); err != nil {
		return fmt.Errorf("can't assign default ipam v6 prefix: %w", err)
	}
	// Create a default security group for the default VPC - all have the same ID
	sg, err := api.createDefaultSecurityGroup(ctx, tx, org.ID, org.ID)
	if err != nil {
		return fmt.Errorf("failed to create the default security group: %w", res.Error)
	}

	// Update the default org with the new security group id
	if err := api.updateVpcSecGroupId(ctx, tx, sg.ID, org.ID); err != nil {
		return fmt.Errorf("failed to create the default organization with a security group id: %w", res.Error)
	}

	return nil
}

// GetUser gets a user
// @Summary      Get User
// @Description  Gets a user
// @Id           GetUser
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id  path       string  true  "User ID"
// @Success      200  {object}  models.User
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/users/{id} [get]
func (api *API) GetUser(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetUser",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()

	var userId uuid.UUID
	var err error
	if c.Param("id") == "me" {
		userId = api.GetCurrentUserID(c)
	} else {
		userId, err = uuid.Parse(c.Param("id"))
		if err != nil || userId == uuid.Nil {
			c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
			return
		}
	}

	var user models.User
	db := api.db.WithContext(ctx)
	if res := api.UserIsCurrentUser(c, db).
		First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("user"))
		return
	}
	c.JSON(http.StatusOK, user)
}

// ListUsers lists users
// @Summary      List Users
// @Description  Lists all users
// @Id           ListUsers
// @Tags         Users
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.User
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/users [get]
func (api *API) ListUsers(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListUsers")
	defer span.End()
	users := make([]*models.User, 0)
	db := api.db.WithContext(ctx)
	db = api.UserIsCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.User{}, c, "user_name")
	result := db.Find(&users)

	if result.Error != nil {
		api.SendInternalServerError(c, errors.New("error fetching keys from db"))
		return
	}
	c.JSON(http.StatusOK, users)
}

// DeleteUser delete a user
// @Summary      Delete User
// @Description  Delete a user
// @Id           DeleteUser
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id  path       string  true  "User ID"
// @Success      200  {object}  models.User
// @Failure		 400  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/users/{id} [delete]
func (api *API) DeleteUser(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteUser")
	defer span.End()
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var user models.User
	err := api.transaction(ctx, func(tx *gorm.DB) error {
		if res := api.UserIsCurrentUser(c, tx).
			First(&user, "id = ?", userID); res.Error != nil {
			return errUserNotFound
		}
		if res := api.db.Select(clause.Associations).Delete(&user); res.Error != nil {
			api.SendInternalServerError(c, fmt.Errorf("failed to delete user: %w", res.Error))
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("user"))
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	// delete the cached user
	prefixId := fmt.Sprintf("%s:%s", CachePrefix, user.IdpID)
	_, err = api.Redis.Del(c.Request.Context(), prefixId).Result()
	if err != nil {
		api.logger.Warnf("failed to delete the cache user:%s", err)
	}
	c.JSON(http.StatusOK, user)
}

type UserOrganization struct {
	UserID         string    `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
}

// DeleteUserFromOrganization removes a user from an organization
// @Summary      Remove a User from an Organization
// @Description  Deletes an existing organization associated to a user
// @Id			 DeleteUserFromOrganization
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id             path      string  true "User ID"
// @Param        organization   path      string  true "Organization ID"
// @Success      204  {object}  models.User
// @Failure      400  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/users/{id}/organizations/{organization} [delete]
func (api *API) DeleteUserFromOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteUser")
	defer span.End()
	userID := c.Param("id")

	if userID == "" {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	orgID := c.Param("organization")
	if userID == "" {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var user models.User
	var organization models.Organization
	err := api.transaction(ctx, func(tx *gorm.DB) error {
		if res := api.db.First(&user, "id = ?", userID); res.Error != nil {
			return errUserNotFound
		}
		if res := api.db.First(&organization, "id = ?", orgID); res.Error != nil {
			return errOrgNotFound
		}
		if res := api.db.
			Select(clause.Associations).
			Where("user_id = ?", userID).
			Where("organization_id = ?", orgID).
			Delete(&UserOrganization{}); res.Error != nil {
			api.SendInternalServerError(c, fmt.Errorf("failed to remove the association from the user_organizations table: %w", res.Error))
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("user"))
		}
		if errors.Is(err, errOrgNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}
	// delete the cached user
	prefixId := fmt.Sprintf("%s:%s", CachePrefix, user.IdpID)
	_, err = api.Redis.Del(c.Request.Context(), prefixId).Result()
	if err != nil {
		api.logger.Warnf("failed to delete the cache user:%s", err)
	}
	c.JSON(http.StatusOK, user)
}
