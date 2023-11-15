package handlers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
	"net/http"
	"time"
)

// GarbageCollect cleans up old soft deleted records
// @Summary      Cleans up old soft deleted records
// @Description  Cleans up old soft deleted records
// @Id           GarbageCollect
// @Tags         Private
// @Accept       json
// @Produce      json
// @Param		 retention   query    string false "how long to retain deleted records.  defaults to '24h'"
// @Success      204
// @Failure      400  {object}  models.ValidationError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /admin/gc [post]
func (api *API) GarbageCollect(c *gin.Context) {
	ctx, span := tracer.Start(c, "GarbageCollect")
	defer span.End()
	query := struct {
		Retention string `form:"retention"`
	}{}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("after"))
		return
	}
	if query.Retention == "" {
		query.Retention = "24h"
	}

	d, err := time.ParseDuration(query.Retention)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewFieldValidationError("after", fmt.Sprintf("must be a valid duration: %s", err)))
		return
	}

	db := api.db.WithContext(ctx)
	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.Invitation{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.RegKey{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.DeviceMetadata{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.Device{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.SecurityGroup{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.VPC{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.Organization{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	err = db.Unscoped().
		Debug().
		Where("deleted_at < ?", time.Now().Add(-d)).
		Delete(&models.User{}).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}
	c.Status(http.StatusNoContent)
}
