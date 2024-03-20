package handlers

import (
	"errors"
	//"fmt"
	//"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"net/http"
	//"time"
	//"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	//"github.com/google/uuid"
	//"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	//"github.com/nexodus-io/nexodus/internal/util"
	//"github.com/nexodus-io/nexodus/internal/wgcrypto"
	//"go.opentelemetry.io/otel/attribute"
	//"go.opentelemetry.io/otel/trace"
	//"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
	//"gorm.io/gorm/clause"
)

// CreateStatus handles adding a new device
// @Summary      Add Status
// @Id  		 CreateStatus
// @Tags         status
// @Description  Adds a new status
// @Accept       json
// @Produce      json
// @Param        Status  body   models.Status  true "Status"
// @Success      201  {object}  models.Status
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/status [post]
func (api *API) CreateStatus(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "AddStatus")
	defer span.End()

	if c.Request.Method != http.MethodPost {
		// Respond with a 405 Method Not Allowed error
		c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{"error": "Method Not Allowed"})
		return
	}

	var request models.Status
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	userId := api.GetCurrentUserID(c)
	var status models.Status

	err := api.transaction(ctx, func(tx *gorm.DB) error {

		status = models.Status{
			UserId:      userId,
			WgIP:        request.WgIP,
			IsReachable: request.IsReachable,
			Hostname:    request.Hostname,
			Latency:     request.Latency,
			Method:      request.Method,
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

	c.JSON(http.StatusCreated, status)

}

func (api *API) GetStatus(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListStatus", trace.WithAttributes(
		attribute.String("UserId", c.Param("UserIdid")),
	))
	defer span.End()

	if !api.FlagCheck(c, "status") {
		return
	}

	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var status models.Status

	db := api.db.WithContext(ctx)
	db = api.StatusIsOwnedByCurrentUser(c, db)
	result := db.First(&status, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (api *API) StatusIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	return db.Where("owner_id = ?", userId)
}

func (api *API) UpdateStatus(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateStatus")
	defer span.End()

	statusId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	var request struct {
		Latency string `json:"latency"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	userId := api.GetCurrentUserID(c)

	err = api.transaction(ctx, func(tx *gorm.DB) error {
		var status models.Status
		if err := tx.Where("id = ? AND user_id = ?", statusId, userId).First(&status).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("status not found or not owned by user")
			}
			return err
		}

		status.Latency = request.Latency
		return tx.Save(&status).Error
	})

	if err != nil {
		if err.Error() == "status not found or not owned by user" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Latency updated successfully"})
}
