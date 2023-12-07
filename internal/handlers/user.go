package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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

func (api *API) CreateUserIfNotExists(ctx context.Context, idpId string, userName string, claimsMap map[string]interface{}) (uuid.UUID, error) {

	ctx, span := tracer.Start(ctx, "CreateUserIfNotExists", trace.WithAttributes(
		attribute.String("ipd_id", idpId),
		attribute.String("username", userName),
	))
	defer span.End()
	var user models.User

	claims := struct {
		Issuer        string `json:"iss"`
		EmailVerified bool   `json:"email_verified"`
		Email         string `json:"email"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
	}{}

	fullName := ""
	if claimsMap != nil {
		err := util.JsonUnmarshal(claimsMap, &claims)
		if err != nil {
			return uuid.Nil, err
		}

		claims.Email = strings.TrimSpace(strings.ToLower(claims.Email))

		names := []string{}
		if claims.GivenName != "" {
			names = append(names, claims.GivenName)
		}
		if claims.FamilyName != "" {
			names = append(names, claims.FamilyName)
		}
		fullName = strings.Join(names, " ")
	}

	fmt.Println("claims", claims)

	// Retry the operation if we get a duplicate key error which can occur on concurrent requests when creating a user
	err := util.RetryOperationForErrors(ctx, time.Millisecond*10, 1, []error{gorm.ErrDuplicatedKey}, func() error {
		return api.transaction(ctx, func(tx *gorm.DB) error {

			if res := tx.First(&user, "idp_id = ?", idpId); res.Error != nil {
				if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
					return res.Error
				}
			} else {
				// The user exists... Do we need to update the user due to a change in the claims?
				if user.UserName != userName || user.FullName != fullName || user.Picture != claims.Picture {
					if res := tx.Model(&user).
						Where("id = ?", user.ID).
						Updates(map[string]interface{}{
							"user_name": userName,
							"full_name": fullName,
							"picture":   claims.Picture,
						}); res.Error != nil {
						api.logger.Warn("failed to update user:", res.Error)
					}
				}

				if claims.EmailVerified && claims.Email != "" {
					// is the email address already associated with the user?
					var count int64
					if res := tx.Model(&models.UserIdentity{}).
						Where("kind = 'email' and value = ? and user_id =?", claims.Email, user.ID).
						Count(&count); res.Error != nil {
						return res.Error
					}
					if count != 1 {
						fmt.Println("adding email")
						// create the identity
						if res := tx.Create(&models.UserIdentity{
							Kind:   "email",
							Value:  claims.Email,
							UserID: user.ID,
						}); res.Error != nil {
							if !database.IsDuplicateError(res.Error) {
								return NewApiResponseError(http.StatusForbidden, models.NewApiError(fmt.Errorf("user already exists with email address: %s", claims.Email)))
							}
							api.logger.Warn("failed to add user email:", res.Error)
						}

						// associate any pending org invites with the user record.
						if res := tx.Model(&models.Invitation{}).
							Where("user_id IS NULL and email = ?", claims.Email).
							Update("user_id", user.ID); res.Error != nil {
							api.logger.Warn("failed to update invitation:", res.Error)
						}
					}
				}

				return nil
			}

			// Create the user
			user.ID = uuid.New()
			user.IdpID = idpId
			user.UserName = userName
			user.Picture = claims.Picture
			user.FullName = fullName
			if res := tx.Create(&user); res.Error != nil {
				if database.IsDuplicateError(res.Error) {
					res.Error = gorm.ErrDuplicatedKey
				}
				return res.Error
			}

			// doing the following should allow us to eventually
			// remove the need for the user.IdpID field, and to
			// associate multiple IDP logins per user
			if res := tx.Create(&models.UserIdentity{
				Kind:   "keycloak:id",
				Value:  idpId,
				UserID: user.ID,
			}); res.Error != nil {
				if database.IsDuplicateError(res.Error) {
					res.Error = gorm.ErrDuplicatedKey
				}
				return res.Error
			}

			if claims.EmailVerified && claims.Email != "" {

				if res := tx.Create(&models.UserIdentity{
					Kind:   "email",
					Value:  claims.Email,
					UserID: user.ID,
				}); res.Error != nil {
					if database.IsDuplicateError(res.Error) {
						return NewApiResponseError(http.StatusForbidden, models.NewApiError(fmt.Errorf("user already exists with email address: %s", claims.Email)))
					}
					return res.Error
				}

				// associate any pending org invites with the user record.
				if res := tx.Model(&models.Invitation{}).
					Where("user_id IS NULL and email = ?", claims.Email).
					Update("user_id", user.ID); res.Error != nil {
					return res.Error
				}
			}

			// Create the default organization
			if res := tx.Create(&models.Organization{
				Base: models.Base{
					ID: user.ID,
				},
				OwnerID:     user.ID,
				Name:        userName,
				Description: fmt.Sprintf("%s's organization", userName),
				Users: []*models.User{{
					Base: models.Base{
						ID: user.ID,
					},
				}},
			}); res.Error != nil {
				if database.IsDuplicateError(res.Error) {
					res.Error = gorm.ErrDuplicatedKey
				}
				return res.Error
			}

			// Create the default vpc
			if res := tx.Create(&models.VPC{
				Base: models.Base{
					ID: user.ID,
				},
				OrganizationID: user.ID,
				Description:    "default vpc",
				PrivateCidr:    false,
				Ipv4Cidr:       defaultIPAMv4Cidr,
				Ipv6Cidr:       defaultIPAMv6Cidr,
			}); res.Error != nil {
				if database.IsDuplicateError(res.Error) {
					res.Error = gorm.ErrDuplicatedKey
				}
				return res.Error
			}

			// Create a default security group for the default VPC - all have the same ID
			err := api.createDefaultSecurityGroup(ctx, tx, user.ID, user.ID)
			if err != nil {
				return err
			}

			return nil
		})
	})
	if err != nil {
		return uuid.Nil, err
	}
	return user.ID, nil
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
// @Failure		 400  {object}  models.ValidationError
// @Failure      400  {object}  models.NotAllowedError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/users/{id} [delete]
func (api *API) DeleteUser(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteUser")
	defer span.End()
	userId := c.Param("id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var user models.User
	err := api.transaction(ctx, func(tx *gorm.DB) error {
		if res := api.UserIsCurrentUser(c, tx).
			First(&user, "id = ?", userId); res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("user"))
			} else {
				return res.Error
			}
		}

		var count int64
		result := tx.Model(&models.Device{}).Where("owner_id = ?", userId).Count(&count)
		if result.Error != nil {
			return result.Error
		}
		if count > 0 {
			return NewApiResponseError(http.StatusBadRequest, models.NewNotAllowedError("user cannot be deleted while devices owned by the user are still attached"))
		}

		// Cascade delete related records
		if res := tx.Where("owner_id = ?", userId).Delete(&models.RegKey{}); res.Error != nil {
			return result.Error
		}
		if res := tx.Where("user_id = ?", userId).Delete(&models.Invitation{}); res.Error != nil {
			return result.Error
		}

		// We need to delete all the organizations that the user owns
		orgs := []models.Organization{}
		if res := tx.Where("owner_id = ?", userId).
			Find(&orgs); res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}

		for _, org := range orgs {
			err := deleteOrganization(tx, org.ID)
			if err != nil {
				return err
			}
		}

		// delete the related identities
		if res := tx.Where("user_id = ?", userId).
			Delete(&models.UserIdentity{}); res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}

		// delete the cached user
		prefixId := fmt.Sprintf("%s:%s", CachePrefix, user.IdpID)
		_, err := api.Redis.Del(c.Request.Context(), prefixId).Result()
		if err != nil {
			api.logger.Warnf("failed to delete the cache user:%s", err)
		}

		// Null out unique fields so that the user can be created later with the same values
		if res := tx.Model(&user).
			Where("id = ?", userId).
			Updates(map[string]interface{}{
				"idp_id":     nil,
				"deleted_at": gorm.DeletedAt{Time: time.Now(), Valid: true},
			}); res.Error != nil {
			return res.Error
		}

		return nil
	})

	if err != nil {
		var apiResponseError *ApiResponseError
		if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
		} else {
			api.SendInternalServerError(c, err)
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
