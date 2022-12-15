package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// key for username in gin.Context
const AuthUserName string = "_apex.UserName"

func (api *API) CreateUserIfNotExists() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "CreateUserIfNotExists")
		defer span.End()
		id := c.GetString(gin.AuthUserKey)
		userName := c.GetString(AuthUserName)
		var user models.User
		res := api.db.WithContext(ctx).First(&user, "id = ?", id)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				user.ID = id
				user.ZoneID = api.defaultZoneID
				user.Devices = make([]*models.Device, 0)
				user.UserName = userName
				api.db.WithContext(ctx).Create(&user)
			} else {
				_ = c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("can't find record for user id %s", id))
				return
			}
		}
		span.SetAttributes(
			attribute.String("user-id", id),
			attribute.String("username", userName),
		)
		// Check if the UserName has changed since the last time we saw this user
		if user.UserName != userName {
			api.db.WithContext(ctx).Model(&user).Update("UserName", userName)
		}
		c.Next()
	}
}

// PatchUser handles moving a User to a new Zone
// @Summary      Update User
// @Description  Changes the users zone
// @Tags         User
// @Accepts		 json
// @Produce      json
// @Param        id  path       string  true  "User ID"
// @Param        device  body   models.PatchUser  true "Patch User"
// @Success      200  {object}  models.User
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Router       /users/{id} [patch]
func (api *API) PatchUser(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "PatchUser",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	userId := c.Param("id")
	if userId == "" {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "user id is not valid"})
		return
	}
	var request models.PatchUser
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: err.Error()})
		return
	}

	var user models.User
	if userId == "me" {
		userId = c.GetString(gin.AuthUserKey)
	}

	if res := api.db.WithContext(ctx).First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: res.Error.Error()})
		return
	}

	var zone models.Zone
	if res := api.db.WithContext(ctx).First(&zone, "id = ?", request.ZoneID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "zone id is not valid"})
		return
	}

	user.ZoneID = request.ZoneID

	api.db.WithContext(ctx).Save(&user)

	c.JSON(http.StatusOK, user)
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
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "user id is not valid"})
		return
	}

	var user models.User
	if userId == "me" {
		userId = c.GetString(gin.AuthUserKey)
	}

	if res := api.db.WithContext(ctx).Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusNotFound, models.ApiError{Error: res.Error.Error()})
		return
	}

	var devices []uuid.UUID
	for _, d := range user.Devices {
		devices = append(devices, d.ID)
	}
	user.DeviceList = devices

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
	result := api.db.WithContext(ctx).Preload("Devices").Scopes(FilterAndPaginate(&models.User{}, c)).Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	for _, u := range users {
		var devices []uuid.UUID
		for _, d := range u.Devices {
			devices = append(devices, d.ID)
		}
		u.DeviceList = devices
	}
	c.JSON(http.StatusOK, users)
}
