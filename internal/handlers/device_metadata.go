package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
)

func metadataForDevice(deviceId string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("device_id = ?", deviceId)
	}
}

// ListDeviceMetadata lists metadata for a device
// @Summary      List Device Metadata
// @Id  		 ListDeviceMetadata
// @Tags         Devices
// @Description  Lists metadata for a device
// @Param        id          path   string  true  "Device ID"
// @Param		 gt_revision query  uint64  false "greater than revision"
// @Accept	     json
// @Produce      json
// @Success      200  {object}  []models.DeviceMetadata
// @Failure      500  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata [get]
func (api *API) ListDeviceMetadata(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListDeviceMetadata", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	deviceId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	query := struct {
		Query
		Prefixes []string `form:"prefix"`
	}{}
	if err := c.BindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(err))
		return
	}

	var device models.Device
	result := api.db.WithContext(ctx).
		Scopes(api.DeviceIsOwnedByCurrentUser(c)).
		First(&device, "id = ?", deviceId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		api.logger.Errorf("error fetching metadata: %s", result.Error)
		c.JSON(http.StatusInternalServerError, result.Error)
		return
	}

	defaultOrderBy := "key"

	scopes := []func(*gorm.DB) *gorm.DB{
		func(db *gorm.DB) *gorm.DB {
			result := db.Where(
				db.Where("device_id = ?", deviceId.String()),
			)
			if len(query.Prefixes) > 0 {
				oredExpressions := db
				for i, prefix := range query.Prefixes {
					if i == 0 {
						oredExpressions = oredExpressions.Where("key LIKE ?", prefix+"%")
					} else {
						oredExpressions = oredExpressions.Or("key LIKE ?", prefix+"%")
					}
				}
				result = result.Where(
					oredExpressions,
				)
			}
			return result
		},
		FilterAndPaginateWithQuery(&models.DeviceMetadata{}, c, query.Query, defaultOrderBy),
	}
	api.sendList(c, ctx, getDeviceMetadataList, scopes)
}

func getDeviceMetadataList(db *gorm.DB) (WatchableList, error) {
	var items deviceMetadataList
	result := db.Find(&items)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, result.Error
	}
	return items, nil
}

// ListOrganizationMetadata lists metadata for all devices in an organization
// @Summary      List Device Metadata
// @Id  		 ListOrganizationMetadata
// @Tags         Devices
// @Description  Lists metadata for a device
// @Param        organization    path   string   true  "Organization ID"
// @Param		 gt_revision     query  uint64   false "greater than revision"
// @Param        prefix          path   []string true  "used to filter down to the specified key prefixes"
// @Accept	     json
// @Produce      json
// @Success      200  {object}  []models.DeviceMetadata
// @Failure      500  {object}  models.BaseError
// @Router       /api/organizations/{organization}/metadata [get]
func (api *API) ListOrganizationMetadata(c *gin.Context) {
	orgIdParam := c.Param("organization")
	ctx, span := tracer.Start(c.Request.Context(), "ListOrganizationMetadata", trace.WithAttributes(
		attribute.String("organization", orgIdParam),
	))
	defer span.End()

	orgId, err := uuid.Parse(orgIdParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	query := struct {
		Query
		Prefixes []string `form:"prefix"`
	}{}
	if err := c.BindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiInternalError(err))
		return
	}

	var org models.Organization
	result := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsReadableByCurrentUser(c)).
		First(&org, "id = ?", orgId.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(result.Error))
		}
		return
	}

	signalChannel := fmt.Sprintf("/metadata/org=%s", orgId.String())
	defaultOrderBy := "key"
	if v := c.Query("watch"); v == "true" {
		query.Sort = ""
		defaultOrderBy = "revision"
	}

	scopes := []func(*gorm.DB) *gorm.DB{
		func(db *gorm.DB) *gorm.DB {
			result := db.Model(&models.DeviceMetadata{}).
				Joins("inner join devices on devices.id=device_metadata.device_id").
				Where( // extra wrapping Where needed to group the SQL expressions
					db.Where("devices.organization_id = ?", orgId.String()),
				)
			if len(query.Prefixes) > 0 {
				oredExpressions := db
				for i, prefix := range query.Prefixes {
					if i == 0 {
						oredExpressions = oredExpressions.Where("key LIKE ?", prefix+"%")
					} else {
						oredExpressions = oredExpressions.Or("key LIKE ?", prefix+"%")
					}
				}
				result = result.Where( // extra wrapping Where needed to group the SQL expressions
					oredExpressions,
				)
			}
			return result
		},
		FilterAndPaginateWithQuery(&models.DeviceMetadata{}, c, query.Query, defaultOrderBy),
	}

	if len(query.Prefixes) > 0 {
		scopes = append(scopes, func(db *gorm.DB) *gorm.DB {
			for _, prefix := range query.Prefixes {
				db = db.Where("key LIKE ?", prefix+"%")
			}
			return db
		})
	}
	api.sendListOrWatch(c, ctx, signalChannel, "device_metadata.revision", scopes, getDeviceMetadataList)
}

type deviceMetadataList []*models.DeviceMetadata

func (d deviceMetadataList) Item(i int) (any, uint64, gorm.DeletedAt) {
	item := d[i]
	return item, item.Revision, item.DeletedAt
}

func (d deviceMetadataList) Len() int {
	return len(d)
}

// GetDeviceMetadataKey Get value for a metadata key on a device
// @Summary      Get Device Metadata
// @Id  		 GetDeviceMetadataKey
// @Tags         Devices
// @Description  Get metadata for a device
// @Param        id   path      string  true "Device ID"
// @Param        key  path      string  true "Metadata Key"
// @Accept	     json
// @Produce      json
// @Success      200  {object}  models.DeviceMetadata
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata/{key} [get]
func (api *API) GetDeviceMetadataKey(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "GetDeviceMetadataKey", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
		attribute.String("key", c.Param("key")),
	))
	defer span.End()
	deviceId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	key := c.Param("key")

	var metadataInstance models.DeviceMetadata
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		var device models.Device
		result := api.db.WithContext(ctx).
			Scopes(api.DeviceIsOwnedByCurrentUser(c)).
			First(&device, "id = ?", deviceId)
		if result.Error != nil {
			return result.Error
		}

		result = api.db.WithContext(ctx).
			Scopes(metadataForDevice(deviceId.String())).
			Where("key = ?", key).
			First(&metadataInstance)

		return result.Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		api.logger.Errorf("error fetching metadata: %s", err)
		c.JSON(http.StatusInternalServerError, err)
	}

	c.JSON(http.StatusOK, metadataInstance)
}

// UpdateDeviceMetadataKey Set value for a metadata key on a device
// @Summary      Set Device Metadata by key
// @Id  		 UpdateDeviceMetadataKey
// @Tags         Devices
// @Description  Set metadata key for a device
// @Param        id    path      string          true  "Device ID"
// @Param        key   path      string          false "Metadata Key"
// @Param		 value body      any true "Metadata Value"
// @Accept	     json
// @Produce      json
// @Success      200  {object}  models.DeviceMetadata
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata/{key} [put]
func (api *API) UpdateDeviceMetadataKey(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "UpdateDeviceMetadataKey", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
		attribute.String("key", c.Param("key")),
	))
	defer span.End()
	deviceId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	key := c.Param("key")

	var request json.RawMessage
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	metadataInstance := models.DeviceMetadata{
		DeviceID: deviceId,
		Key:      key,
		Value:    request,
	}

	var device models.Device
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		result := api.db.WithContext(ctx).
			Scopes(api.DeviceIsOwnedByCurrentUser(c)).
			First(&device, "id = ?", deviceId)
		if result.Error != nil {
			return result.Error
		}

		result = tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Save(&metadataInstance)
		return result.Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		api.logger.Errorf("error updating metadata: %s", err)
		c.JSON(http.StatusInternalServerError, err)
	}

	signalChannel := fmt.Sprintf("/metadata/org=%s", device.OrganizationID.String())
	api.signalBus.Notify(signalChannel)
	c.JSON(http.StatusOK, metadataInstance)

}

// DeleteDeviceMetadata Delete all metadata or a specific key on a device
// @Summary      Delete all Device metadata
// @Id  		 DeleteDeviceMetadata
// @Tags         Devices
// @Description  Delete all metadata for a device
// @Param        id   path      string  true "Device ID"
// @Success      204
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata [delete]
func (api *API) DeleteDeviceMetadata(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "DeleteDeviceMetadata", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()
	deviceId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var device models.Device
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		result := api.db.WithContext(ctx).
			Scopes(api.DeviceIsOwnedByCurrentUser(c)).
			First(&device, "id = ?", deviceId)
		if result.Error != nil {
			return result.Error
		}

		result = tx.Delete(&models.DeviceMetadata{}, "device_id", deviceId)
		return result.Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		api.logger.Errorf("error deleting metadata: %s", err)
		c.JSON(http.StatusInternalServerError, err)
	}

	signalChannel := fmt.Sprintf("/metadata/org=%s", device.OrganizationID.String())
	api.signalBus.Notify(signalChannel)
	c.Status(http.StatusNoContent)
}

// DeleteDeviceMetadataKey Delete all metadata or a specific key on a device
// @Summary      Delete a Device metadata key
// @Id  		 DeleteDeviceMetadataKey
// @Tags         Devices
// @Description  Delete a metadata key for a device
// @Param        id   path      string  true "Device ID"
// @Param        key  path      string  false "Metadata Key"
// @Success      204
// @Failure      501  {object}  models.BaseError
// @Router       /api/devices/{id}/metadata/{key} [delete]
func (api *API) DeleteDeviceMetadataKey(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteDeviceMetadataKey", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
		attribute.String("key", c.Param("key")),
	))
	defer span.End()
	deviceId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	key := c.Param("key")

	var device models.Device
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		result := api.db.WithContext(ctx).
			Scopes(api.DeviceIsOwnedByCurrentUser(c)).
			First(&device, "id = ?", deviceId)
		if result.Error != nil {
			return result.Error
		}

		result = tx.Delete(&models.DeviceMetadata{
			DeviceID: deviceId,
			Key:      key,
		})
		return result.Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		api.logger.Errorf("error deleting metadata: %s", err)
		c.JSON(http.StatusInternalServerError, err)
	}

	signalChannel := fmt.Sprintf("/metadata/org=%s", device.OrganizationID.String())
	api.signalBus.Notify(signalChannel)
	c.Status(http.StatusNoContent)
}
