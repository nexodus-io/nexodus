package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// CreateStatus handles adding a new status
// @Summary      Add Statuses
// @Id  		 CreateStatus
// @Tags         Statuses
// @Description  Adds a new status
// @Accept       json
// @Produce      json
// @Param        Status  body   models.AddStatus  true "Add Status"
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

	var request models.AddStatus
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	userId := api.GetCurrentUserID(c)
	var status models.Status

	err := api.transaction(ctx, func(tx *gorm.DB) error {
		var user models.User
		if res := tx.First(&user, "id = ?", userId); res.Error != nil {
			return errUserNotFound
		}

		status = models.Status{
			UserId:      api.GetCurrentUserID(c),
			WgIP:        request.WgIP,
			IsReachable: request.IsReachable,
			Hostname:    request.Hostname,
			Latency:     request.Latency,
			Method:      request.Method,
		}

		if res := tx.Create(&status); res.Error != nil {
			if database.IsDuplicateError(res.Error) {
				return res.Error
			}
			api.logger.Error("Failed to create organization: ", res.Error)
			return res.Error
		}

		api.logger.Infof("New Status request [ %s ] request", status.UserId)
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

// GetStatus gets the status of a user by their ID
// @Summary Get user status
// @Description Gets statuses based on userd
// @Tags status
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Param id path string true "Unique identifier for the status"
// @Success      200  {object}  models.Status
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router /status/{id} [get]
func (api *API) GetStatus(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetStatus", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()

	_, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var status models.Status

	db := api.db.WithContext(ctx)
	db = api.StatusIsOwnedByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.Status{}, c, "hostname")
	result := db.Find(&status)

	//result := db.Find(&status, "id = ?", k)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListStatues Lists all statuses
// @Summary      List Statuses
// @Description  Lists all Statuses
// @Id  		 ListStatuses
// @Tags         Statuses
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.Status
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/status [get]
func (api *API) ListStatuses(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListStatuses")
	defer span.End()
	var status []models.Status

	//status := make([]models.Status, 0)

	db := api.db.WithContext(ctx)
	db = api.StatusIsOwnedByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.Status{}, c, "wg_ip")
	result := db.Find(&status)
	if result.Error != nil {
		api.SendInternalServerError(c, errors.New("error fetching statuses"))
		return
	}

	c.JSON(http.StatusOK, status)
}

func (api *API) StatusIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	return db.Where("user_id = ?", userId)
}

/*func (api *API) UpdateStatus(c *gin.Context) {
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
}*/

// DeleteAllStatuses deletes all statuses in the database
// @Summary      Delete All Statuses
// @Description  Deletes all statuses from the database
// @Tags         Statuses
// @Accept       json
// @Produce      json
// @Success      204      "No Content"
// @Failure      401      {object}  models.BaseError
// @Failure      429      {object}  models.BaseError
// @Failure      500      {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/status [delete]
func (api *API) DeleteAllStatuses(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteAllStatuses")
	defer span.End()

	userId := api.GetCurrentUserID(c)

	err := api.transaction(ctx, func(tx *gorm.DB) error {

		if res := tx.Unscoped().Where("user_id = ?", userId).Delete(&models.Status{}); res.Error != nil {
			api.logger.Error("Failed to delete statuses for user: ", res.Error)
			return res.Error
		}
		return nil
	})

	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
