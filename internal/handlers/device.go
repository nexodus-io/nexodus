package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
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
	devices := make([]*models.Device, 0)
	result := api.db.Preload("Peers").Scopes(FilterAndPaginate(&models.Device{}, c)).Find(&devices)
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
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ApiError{Error: "id is not valid"})
		return
	}
	var device models.Device
	result := api.db.Preload("Peers").First(&device, "id = ?", k)
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

// PostDevices handles adding a new device
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
	if res := api.db.Preload("Devices").First(&user, "id = ?", userId); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: "user not found"})
		return
	}

	var device models.Device
	res := api.db.Where("public_key = ?", request.PublicKey).First(&device)
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

	if res := api.db.Create(&device); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.ApiError{Error: res.Error.Error()})
		return
	}

	user.Devices = append(user.Devices, &device)
	api.db.Save(&user)

	c.JSON(http.StatusCreated, device)
}
