package handlers

import (
	"errors"
	"fmt"
	"gorm.io/gorm/clause"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// CreateServiceNetwork creates a new ServiceNetwork
// @Summary      Create an ServiceNetwork
// @Description  Creates a named serviceNetwork with the given CIDR
// @Id			 CreateServiceNetwork
// @Tags         ServiceNetwork
// @Accept       json
// @Produce      json
// @Param        ServiceNetwork  body     models.AddServiceNetwork  true  "Add ServiceNetwork"
// @Success      201  {object}  models.ServiceNetwork
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 405  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/service-networks [post]
func (api *API) CreateServiceNetwork(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateServiceNetwork")
	defer span.End()

	var request models.AddServiceNetwork
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	if request.OrganizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("organization_id"))
		return
	}

	var serviceNetwork models.ServiceNetwork
	err := api.transaction(ctx, func(tx *gorm.DB) error {

		var org models.Organization
		if res := api.OrganizationIsReadableByCurrentUser(c, tx).
			First(&org, "id = ?", request.OrganizationID.String()); res.Error != nil {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("organization"))
		}

		serviceNetwork = models.ServiceNetwork{
			OrganizationID: request.OrganizationID,
			Description:    request.Description,
		}

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Create(&serviceNetwork); res.Error != nil {
			if database.IsDuplicateError(res.Error) {
				return NewApiResponseError(http.StatusConflict, models.NewConflictsError(serviceNetwork.ID.String()))
			}
			return fmt.Errorf("failed to create service_network: %w", res.Error)
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

	c.JSON(http.StatusCreated, serviceNetwork)
}

func (api *API) ServiceNetworkIsReadableByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	return api.CurrentUserHasRole(c, db, "organization_id", MemberRoles)
}

func (api *API) ServiceNetworkIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	return api.CurrentUserHasRole(c, db, "organization_id", OwnerRoles)
}

// ListServiceNetworks lists all ServiceNetworks
// @Summary      List ServiceNetworks
// @Description  Lists all ServiceNetworks
// @Id 			 ListServiceNetworks
// @Tags         ServiceNetwork
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.ServiceNetwork
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/service-networks [get]
func (api *API) ListServiceNetworks(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListServiceNetworks")
	defer span.End()
	var serviceNetworks []models.ServiceNetwork

	db := api.db.WithContext(ctx)
	db = api.ServiceNetworkIsReadableByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.ServiceNetwork{}, c, "description")
	result := db.Find(&serviceNetworks)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("service_network"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, serviceNetworks)
}

// GetServiceNetwork gets a specific ServiceNetwork
// @Summary      Get ServiceNetworks
// @Description  Gets a ServiceNetwork by ServiceNetwork ID
// @Id 			 GetServiceNetwork
// @Tags         ServiceNetwork
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "ServiceNetwork ID"
// @Success      200  {object}  models.ServiceNetwork
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/service-networks/{id} [get]
func (api *API) GetServiceNetwork(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetServiceNetworks",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var serviceNetwork models.ServiceNetwork
	db := api.db.WithContext(ctx)
	result := api.ServiceNetworkIsReadableByCurrentUser(c, db).
		First(&serviceNetwork, "id = ?", id.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("service_network"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, serviceNetwork)
}

// DeleteServiceNetwork handles deleting an existing serviceNetwork
// @Summary      Delete ServiceNetwork
// @Description  Deletes an existing serviceNetwork
// @Id 			 DeleteServiceNetwork
// @Tags         ServiceNetwork
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "ServiceNetwork ID"
// @Success      204  {object}  models.ServiceNetwork
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/service-networks/{id} [delete]
func (api *API) DeleteServiceNetwork(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteServiceNetwork",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var serviceNetwork models.ServiceNetwork
	db := api.db.WithContext(ctx)
	result := api.ServiceNetworkIsOwnedByCurrentUser(c, db).
		First(&serviceNetwork, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("service_network"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	if serviceNetwork.ID == serviceNetwork.OrganizationID {
		c.JSON(http.StatusBadRequest, models.NewNotAllowedError("default service_network cannot be deleted"))
		return
	}

	var count int64
	result = db.Model(&models.Site{}).Where("service_network_id = ?", id).Count(&count)
	if result.Error != nil {
		api.SendInternalServerError(c, result.Error)
		return
	}
	if count > 0 {
		c.JSON(http.StatusBadRequest, models.NewNotAllowedError("service network cannot be deleted while sites are still attached"))
		return
	}

	// Cascade delete related records
	if res := db.Where("service_network_id = ?", id).Delete(&models.RegKey{}); res.Error != nil {
		api.SendInternalServerError(c, res.Error)
		return
	}

	result = db.Delete(&serviceNetwork)
	if result.Error != nil {
		api.SendInternalServerError(c, result.Error)
		return
	}
	c.JSON(http.StatusOK, serviceNetwork)
}

// UpdateServiceNetwork updates a ServiceNetwork
// @Summary      Update ServiceNetworks
// @Description  Updates a serviceNetwork by ID
// @Id  		 UpdateServiceNetwork
// @Tags         ServiceNetwork
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "ServiceNetwork ID"
// @Param		 update body models.UpdateServiceNetwork true "ServiceNetwork Update"
// @Success      200  {object}  models.ServiceNetwork
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/service-networks/{id} [patch]
func (api *API) UpdateServiceNetwork(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateServiceNetwork", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var request models.UpdateServiceNetwork
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	var serviceNetwork models.ServiceNetwork
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		result := api.ServiceNetworkIsOwnedByCurrentUser(c, tx).First(&serviceNetwork, "id = ?", id)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("service_network"))
		}

		if request.Description != nil {
			serviceNetwork.Description = *request.Description
		}

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Save(&serviceNetwork); res.Error != nil {
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

	api.signalBus.Notify(fmt.Sprintf("/serviceNetwork=%s", serviceNetwork.ID.String()))
	c.JSON(http.StatusOK, serviceNetwork)
}

type serviceNetworkList []*models.ServiceNetwork

func (d serviceNetworkList) Item(i int) (any, string, uint64, gorm.DeletedAt) {
	item := d[i]
	return item, item.ID.String(), item.Revision, item.DeletedAt
}

func (d serviceNetworkList) Len() int {
	return len(d)
}
