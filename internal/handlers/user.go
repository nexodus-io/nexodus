package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// key for username in gin.Context
const AuthUserName string = "_nexodus.UserName"

var noUUID = uuid.UUID{}

func (api *API) CreateUserIfNotExists() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetString(gin.AuthUserKey)
		username := c.GetString(AuthUserName)
		_, err := api.createUserIfNotExists(c.Request.Context(), id, username)
		if err != nil {
			_ = c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Next()
	}
}

func (api *API) UserIsCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)

		// this could potentially be driven by rego output
		return db.Where("id = ?", userId)
	}
}

func (api *API) createUserIfNotExists(ctx context.Context, id string, userName string) (uuid.UUID, error) {
	ctx, span := tracer.Start(ctx, "createUserIfNotExists")
	defer span.End()
	span.SetAttributes(
		attribute.String("user-id", id),
		attribute.String("username", userName),
	)
	var user models.User
	var uuid uuid.UUID

	err := api.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First lets check if the user has ever existed in the database
		res := tx.Unscoped().First(&user, "id = ?", id)

		// If the user exists, then lets restore their status in the database
		if res.Error == nil {
			var err error
			uuid, err = api.restoreDeletedUser(ctx, tx, &user, id, userName)
			if err != nil {
				return err
			}

			return nil
		}

		if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("can't find record for user id %s: %w", id, res.Error)
		}

		user.ID = id
		user.UserName = userName
		if res = tx.Create(&user); res.Error != nil {
			return res.Error
		}
		var err error
		uuid, err = api.createUserOrgIfNotExists(ctx, tx, id, userName)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return noUUID, fmt.Errorf("can't create user record: %w", err)
	}

	if uuid != noUUID {
		return uuid, nil
	}

	return noUUID, fmt.Errorf("can't create user record")
}

// restoreDeleteUser will restore a user if they have been deleted
func (api *API) restoreDeletedUser(ctx context.Context, db *gorm.DB, user *models.User, id string, userName string) (uuid.UUID, error) {
	if user == nil {
		return noUUID, errors.New("user is nil or has no ID")
	}

	if db == nil {
		db = api.db.WithContext(ctx)
	}

	// If the user was previously deleted, then lets make them active again
	if user.DeletedAt.Valid {
		user.DeletedAt = gorm.DeletedAt{}
		res := db.Unscoped().Model(&user).Update("DeletedAt", user.DeletedAt)
		if res.Error != nil {
			return noUUID, res.Error
		}
	}

	// Check if the UserName has changed since the last time we saw this user
	if user.UserName != userName {
		res := db.Model(&user).Update("UserName", userName)
		if res.Error != nil {
			return noUUID, res.Error
		}
	}

	return api.createUserOrgIfNotExists(ctx, db, id, userName)
}

func (api *API) createUserOrgIfNotExists(ctx context.Context, db *gorm.DB, userId string, userName string) (uuid.UUID, error) {
	// TODO: Check if we even need this? Seems like we could get rid of this and use the logic in the transaction
	// for returning if Create fails but the organization exists

	// Get the first org the use owns.
	org := models.Organization{}
	res := api.db.Where("owner_id = ?", userId).First(&org)
	if res.Error == nil {
		return org.ID, nil
	}
	if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		return noUUID, res.Error
	}

	org = models.Organization{
		Name:        userName,
		OwnerID:     userId,
		Description: fmt.Sprintf("%s's organization", userName),
		IpCidr:      defaultOrganizationPrefixIPv4,
		IpCidrV6:    defaultOrganizationPrefixIPv6,
		HubZone:     true,
		Users: []*models.User{&models.User{
			ID: userId,
		}},
	}

	if db == nil {
		db = api.db.WithContext(ctx)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		// Create the organization
		if res := tx.Create(&org); res.Error != nil {
			if !errors.Is(res.Error, gorm.ErrDuplicatedKey) {
				return res.Error
			}

			// If we already have an existing organisation then lets just use that one
			if tx.Where("owner_id = ?", userId).First(&org).Error == nil {
				return nil
			}

			return fmt.Errorf("can't create organization record: %w", res.Error)
		}

		// Create namespaces and prefixes
		if err := api.ipam.CreateNamespace(ctx, org.ID); err != nil {
			return fmt.Errorf("failed to create ipam namespace: %w", err)
		}
		if err := api.ipam.AssignPrefix(ctx, org.ID, defaultOrganizationPrefixIPv4); err != nil {
			return fmt.Errorf("can't assign default ipam v4 prefix: %w", err)
		}
		if err := api.ipam.AssignPrefix(ctx, org.ID, defaultOrganizationPrefixIPv6); err != nil {
			return fmt.Errorf("can't assign default ipam v6 prefix: %w", err)
		}
		// Create a default security group for the organization
		sg, err := api.createDefaultSecurityGroup(ctx, tx, org.ID.String())
		if err != nil {
			return fmt.Errorf("failed to create the default security group: %w", res.Error)
		}

		// Update the default org with the new security group id
		if err := api.updateOrganizationSecGroupId(ctx, tx, sg.ID, org.ID); err != nil {
			return fmt.Errorf("failed to create the default organization with a security group id: %w", res.Error)
		}

		return nil
	})

	if err != nil {
		return noUUID, err
	}

	return org.ID, nil
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
// @Failure      500  {object}  models.BaseError
// @Router       /api/users/{id} [get]
func (api *API) GetUser(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetUser",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	userId := c.Param("id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var user models.User
	if userId == "me" {
		userId = c.GetString(gin.AuthUserKey)
	}

	if res := api.db.WithContext(ctx).
		Scopes(api.UserIsCurrentUser(c)).
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
// @Router       /api/users [get]
func (api *API) ListUsers(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListUsers")
	defer span.End()
	users := make([]*models.User, 0)
	result := api.db.WithContext(ctx).
		Scopes(api.UserIsCurrentUser(c)).
		Scopes(FilterAndPaginate(&models.User{}, c, "user_name")).
		Find(&users)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
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
// @Failure      500  {object}  models.BaseError
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
		if res := api.db.
			Scopes(api.UserIsCurrentUser(c)).
			First(&user, "id = ?", userID); res.Error != nil {
			return errUserNotFound
		}
		if res := api.db.Select(clause.Associations).Delete(&user); res.Error != nil {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to delete user: %w", res.Error)))
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("user"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
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
// @Failure      500  {object}  models.BaseError
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
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(fmt.Errorf("failed to remove the association from the user_organizations table: %w", res.Error)))
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
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}

	c.JSON(http.StatusOK, user)
}
