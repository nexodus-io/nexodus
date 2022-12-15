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

// ListDevices lists all devices
// @Summary      List Devices
// @Description  Lists all devices
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.Device
// @Failure		 401  {object}  models.ApiError
// @Router       /devices [get]
func (api *API) ListDevices(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDevices")
	defer span.End()
	devices := make([]*models.Device, 0)
	result := api.db.WithContext(ctx).Preload("Peers").Scopes(FilterAndPaginate(&models.Device{}, c)).Find(&devices)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	for _, d := range devices {
		peers := make([]uuid.UUID, 0)
		for _, p := range d.Peers {
			peers = append(peers, p.ID)
		}
		d.PeerList = peers
	}
	c.JSON(http.StatusOK, devices)
}

// GetDevice gets a device by ID
// @Summary      Get Devices
// @Description  Gets a device by ID
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      200  {object}  models.Device
// @Failure		 401  {object}  models.ApiError
// @Failure      400  {object}  models.ApiError
// @Failure      404  {object}  models.ApiError
// @Router       /devices/{id} [get]
func (api *API) GetDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetDevice", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "id is not valid"})
		return
	}
	var device models.Device
	result := api.db.WithContext(ctx).Preload("Peers").First(&device, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	peers := make([]uuid.UUID, 0)
	for _, p := range device.Peers {
		peers = append(peers, p.ID)
	}
	device.PeerList = peers
	c.JSON(http.StatusOK, device)
}

// CreateDevice handles adding a new device
// @Summary      Add Devices
// @Description  Adds a new device
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        device  body   models.AddDevice  true "Add Device"
// @Success      201  {object}  models.Device
// @Failure      400  {object}  models.ApiError
// @Failure		 401  {object}  models.ApiError
// @Failure      409  {object}  models.Device
// @Failure      500  {object}  models.ApiError
// @Router       /devices [post]
func (api *API) CreateDevice(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateDevice")
	defer span.End()
	var request models.AddDevice
	// Call BindJSON to bind the received JSON
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: err.Error()})
		return
	}
	if request.PublicKey == "" {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "the request did not contain a valid public key"})
		return
	}

	userId := c.GetString(gin.AuthUserKey)
	var user models.User
	if res := api.db.WithContext(ctx).Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "user not found"})
		return
	}

	var device models.Device
	res := api.db.WithContext(ctx).Where("public_key = ?", request.PublicKey).First(&device)
	if res.Error == nil {
		c.JSON(http.StatusConflict, device)
		return
	}
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "database error"})
		return
	}

	device = models.Device{
		PublicKey: request.PublicKey,
		UserID:    user.ID,
		Hostname:  request.Hostname,
	}

	if res := api.db.WithContext(ctx).Create(&device); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: res.Error.Error()})
		return
	}
	span.SetAttributes(
		attribute.String("id", device.ID.String()),
	)
	user.Devices = append(user.Devices, &device)
	api.db.WithContext(ctx).Save(&user)

	c.JSON(http.StatusCreated, device)
}

// DeleteDevice handles deleting an existing device and associated ipam lease
// @Summary      Delete Device
// @Description  Deletes an existing device and associated IPAM lease
// @Tags         Devices
// @Accepts		 json
// @Produce      json
// @Param        id   path      string  true "Device ID"
// @Success      204  {object}  models.Device
// @Failure      400  {object}  models.ApiError
// @Failure		 400  {object}  models.ApiError
// @Failure		 400  {object}  models.ApiError
// @Failure      400  {object}  models.ApiError
// @Failure      500  {object}  models.ApiError
// @Router       /devices/{id} [delete]
func (api *API) DeleteDevice(c *gin.Context) {
	deviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "device id is not valid"})
		return
	}

	var peer models.Peer
	baseID := models.Base{ID: deviceID}
	device := models.Device{}
	device.Base = baseID

	if res := api.db.First(&peer, "device_id = ?", device.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: res.Error.Error()})
		return
	}
	ipamAddress := peer.NodeAddress
	zonePrefix := peer.ZonePrefix

	if res := api.db.Delete(&peer, "device_id = ?", device.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: res.Error.Error()})
		return
	}

	if res := api.db.Delete(&device, "id = ?", device.Base.ID); res.Error != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: res.Error.Error()})
		return
	}

	if ipamAddress != "" && zonePrefix != "" {
		if err := api.ipam.ReleaseToPool(c.Request.Context(), ipamAddress, zonePrefix); err != nil {
			c.JSON(http.StatusInternalServerError, models.ApiError{
				Error: fmt.Sprintf("failed to release ipam address: %v", err),
			})
		}
	}

	c.JSON(http.StatusOK, device)
}
