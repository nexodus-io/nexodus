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
)

// key for username in gin.Context
const AuthUserName string = "_apex.UserName"

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

func (api *API) createUserIfNotExists(ctx context.Context, id string, userName string) (uuid.UUID, error) {
	ctx, span := tracer.Start(ctx, "createUserIfNotExists")
	defer span.End()
	var user models.User
	err := api.transaction(ctx, func(tx *gorm.DB) error {
		res := tx.Preload("Organizations").First(&user, "id = ?", id)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				user.ID = id
				user.UserName = userName
				user.Organizations = []*models.Organization{
					{
						Name:        userName,
						Description: fmt.Sprintf("%s's organization", userName),
						IpCidr:      defaultOrganizationPrefix,
						HubZone:     true,
					},
				}
				if res := tx.Create(&user); res.Error != nil {
					return fmt.Errorf("can't create user record: %w", res.Error)
				}
				if err := api.ipam.CreateNamespace(ctx, user.Organizations[0].ID); err != nil {
					return fmt.Errorf("failed to create ipam namespace: %w", err)
				}
				if err := api.ipam.AssignPrefix(ctx, user.Organizations[0].ID, defaultOrganizationPrefix); err != nil {
					return fmt.Errorf("can't assign default ipam prefix: %w", err)
				}
			} else {
				return fmt.Errorf("can't find record for user id %s", id)
			}
		}
		span.SetAttributes(
			attribute.String("user-id", id),
			attribute.String("username", userName),
		)
		// Check if the UserName has changed since the last time we saw this user
		if user.UserName != userName {
			tx.Model(&user).Update("UserName", userName)
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return user.Organizations[0].ID, nil
}

// GetUser gets a user
// @Summary      Get User
// @Description  Gets a user
// @Tags         User
// @Accepts		 json
// @Produce      json
// @Param        id  path       string  true  "User ID"
// @Success      200  {object}  models.User
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Router       /users/{id} [get]
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

	if res := api.db.WithContext(ctx).Preload("Devices").Preload("Organizations").First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("user"))
		return
	}
	c.JSON(http.StatusOK, user)
}

// ListUsers lists users
// @Summary      List Users
// @Description  Lists all users
// @Tags         User
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.User
// @Failure		 401  {object}  models.ApiError
// @Router       /users [get]
func (api *API) ListUsers(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListUsers")
	defer span.End()
	users := make([]*models.User, 0)
	result := api.db.WithContext(ctx).Preload("Devices").Preload("Organizations").Scopes(FilterAndPaginate(&models.User{}, c)).Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	c.JSON(http.StatusOK, users)
}

// DeleteUser delete a user
// @Summary      Delete User
// @Description  Delete a user
// @Tags         User
// @Accepts		 json
// @Produce      json
// @Param        id  path       string  true  "User ID"
// @Success      200  {object}  models.User
// @Failure		 400  {object}  models.ApiError
// @Failure      400  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Router       /users/{id} [delete]
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
		if res := api.db.First(&user, "id = ?", userID); res.Error != nil {
			return errUserNotFound
		}
		if res := api.db.Delete(&user, "id = ?", userID); res.Error != nil {
			return res.Error
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
