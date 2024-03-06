package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
)

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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(err))
		return
	}

	var device models.Device
	db := api.db.WithContext(ctx)
	result := api.DeviceIsOwnedByCurrentUser(c, db).
		First(&device, "id = ?", deviceId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		api.SendInternalServerError(c, fmt.Errorf("error fetching metadata: %w", result.Error))
		return
	}

	api.sendList(c, ctx, func(db *gorm.DB) (fetchmgr.ResourceList, error) {

		tempDB := db.Where(
			db.Where("device_id = ?", deviceId.String()),
		)

		// Building OR expressions with gorm is tricky...
		if len(query.Prefixes) > 0 {
			expressions := db
			for i, prefix := range query.Prefixes {
				if i == 0 {
					expressions = expressions.Where("key LIKE ?", prefix+"%")
				} else {
					expressions = expressions.Or("key LIKE ?", prefix+"%")
				}
			}
			tempDB = tempDB.Where( // extra wrapping Where needed to group the SQL expressions
				expressions,
			)
		}
		db = tempDB
		db = FilterAndPaginateWithQuery(db, &models.DeviceMetadata{}, c, query.Query, "key")

		var items deviceMetadataList
		result := db.Find(&items)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
		return items, nil
	})
}

// ListMetadataInVPC lists metadata for all devices in the vpc
// @Summary      List Device Metadata
// @Id  		 ListMetadataInVPC
// @Tags         VPC
// @Description  Lists metadata for a device
// @Param        id              path   string   true  "VPC ID"
// @Param		 gt_revision     query  uint64   false "greater than revision"
// @Param        prefix          query   []string false  "used to filter down to the specified key prefixes"
// @Param        key             query   string false  "used to filter down to the specified key"
// @Accept	     json
// @Produce      json
// @Success      200  {object}  []models.DeviceMetadata
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/vpcs/{id}/metadata [get]
func (api *API) ListMetadataInVPC(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListMetadataInVPC",
		trace.WithAttributes(
			attribute.String("vpc_id", c.Param("id")),
		))
	defer span.End()

	vpcId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	query := struct {
		Query
		Prefixes []string `form:"prefix"`
		Key      string   `form:"key"`
	}{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(err))
		return
	}

	var vpc models.VPC
	db := api.db.WithContext(ctx)
	result := api.VPCIsReadableByCurrentUser(c, db).
		First(&vpc, "id = ?", vpcId.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	api.sendList(c, ctx, func(db *gorm.DB) (fetchmgr.ResourceList, error) {

		tempDB := db.Model(&models.DeviceMetadata{}).
			Joins("inner join devices on devices.id=device_metadata.device_id").
			Where( // extra wrapping Where needed to group the SQL expressions
				db.Where("devices.vpc_id = ?", vpcId.String()),
			)

		// Building OR expressions with gorm is tricky...
		if len(query.Prefixes) > 0 {
			expressions := db
			for i, prefix := range query.Prefixes {
				if i == 0 {
					expressions = expressions.Where("key LIKE ?", prefix+"%")
				} else {
					expressions = expressions.Or("key LIKE ?", prefix+"%")
				}
			}
			tempDB = tempDB.Where( // extra wrapping Where needed to group the SQL expressions
				expressions,
			)
		}
		db = tempDB

		if query.Key != "" {
			db = db.Where("key = ?", query.Key)
		}
		db = FilterAndPaginateWithQuery(db, &models.DeviceMetadata{}, c, query.Query, "key")

		var items deviceMetadataList
		result := db.Find(&items)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
		return items, nil
	})
}

type deviceMetadataList []*models.DeviceMetadata

func (d deviceMetadataList) Item(i int) (any, string, uint64, gorm.DeletedAt) {
	item := d[i]
	return item, fmt.Sprintf("%s/%s", item.DeviceID, item.Key), item.Revision, item.DeletedAt
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
		db := api.db.WithContext(ctx)
		result := api.DeviceIsOwnedByCurrentUser(c, db).
			First(&device, "id = ?", deviceId)
		if result.Error != nil {
			return result.Error
		}

		result = db.Where("device_id = ?", deviceId.String()).
			Where("key = ?", key).
			First(&metadataInstance)

		return result.Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		api.SendInternalServerError(c, fmt.Errorf("error fetching metadata: %w", err))
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	metadataInstance := models.DeviceMetadata{
		DeviceID: deviceId,
		Key:      key,
		Value:    request,
	}

	var device models.Device
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		result := api.DeviceIsOwnedByCurrentUser(c, tx).
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
		api.SendInternalServerError(c, fmt.Errorf("error updating metadata: %w", err))
		return
	}

	signalChannel := fmt.Sprintf("/metadata/vpc=%s", device.VpcID.String())
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
		result := api.DeviceIsOwnedByCurrentUser(c, tx).
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
		api.SendInternalServerError(c, fmt.Errorf("error deleting metadata: %w", err))
	}

	signalChannel := fmt.Sprintf("/metadata/vpc=%s", device.VpcID.String())
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
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
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
		result := api.DeviceIsOwnedByCurrentUser(c, tx).
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
		api.SendInternalServerError(c, fmt.Errorf("error deleting metadata: %w", err))
	}

	signalChannel := fmt.Sprintf("/metadata/vpc=%s", device.VpcID.String())
	api.signalBus.Notify(signalChannel)
	c.Status(http.StatusNoContent)
}
