package handlers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"net/http"
	"time"
)

type errDuplicateOrganization struct {
	ID string
}

func (e errDuplicateOrganization) Error() string {
	return "organization already exists"
}

// CreateOrganization creates a new Organization
// @Summary      Create an Organization
// @Description  Creates a named organization with the given CIDR
// @Id			 CreateOrganization
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        Organization  body     models.AddOrganization  true  "Add Organization"
// @Success      201  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 405  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations [post]
func (api *API) CreateOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateOrganization")
	defer span.End()

	if !api.FlagCheck(c, "multi-organization") {
		return
	}

	userId := api.GetCurrentUserID(c)

	var request models.AddOrganization
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	if request.Name == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("name"))
		return
	}

	var org models.Organization
	err := api.transaction(ctx, func(tx *gorm.DB) error {
		var user models.User
		if res := tx.First(&user, "id = ?", userId); res.Error != nil {
			return errUserNotFound
		}

		org = models.Organization{
			Name:        request.Name,
			Description: request.Description,
		}

		if res := tx.Create(&org); res.Error != nil {
			if database.IsDuplicateError(res.Error) {
				return errDuplicateOrganization{ID: org.ID.String()}
			}
			api.logger.Error("Failed to create organization: ", res.Error)
			return res.Error
		}

		if res := tx.Create(&models.UserOrganization{
			UserID:         userId,
			OrganizationID: org.ID,
			Roles:          []string{"owner"},
		}); res.Error != nil {
			if database.IsDuplicateError(res.Error) {
				return errDuplicateOrganization{ID: org.ID.String()}
			}
			api.logger.Error("Failed to create organization owner: ", res.Error)
			return res.Error
		}

		span.SetAttributes(attribute.String("id", org.ID.String()))
		api.logger.Infof("New organization request [ %s ] request", org.Name)
		return nil
	})

	if err != nil {
		var duplicate errDuplicateOrganization
		if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewApiError(err))
		} else if errors.As(err, &duplicate) {
			c.JSON(http.StatusConflict, models.NewConflictsError(duplicate.ID))
		} else {
			api.SendInternalServerError(c, err)
		}
		return
	}

	c.JSON(http.StatusCreated, org)
}

func (api *API) OrganizationIsReadableByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	allowedRoles := []string{"owner", "member"}
	if api.dialect == database.DialectSqlLite {
		return db.Where("id in (SELECT DISTINCT organization_id FROM user_organizations, json_each(roles) AS role where user_id=? AND role.value IN (?))", userId, allowedRoles)
	} else {
		return db.Where("id in (SELECT DISTINCT organization_id FROM user_organizations where user_id=? AND (roles && ?))", userId, models.StringArray(allowedRoles))
	}
}

func (api *API) OrganizationIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	allowedRoles := []string{"owner"}
	if api.dialect == database.DialectSqlLite {
		return db.Where("id in (SELECT DISTINCT organization_id FROM user_organizations, json_each(roles) AS role where user_id=? AND role.value IN (?))", userId, allowedRoles)
	} else {
		return db.Where("id in (SELECT DISTINCT organization_id FROM user_organizations where user_id=? AND (roles && ?))", userId, models.StringArray(allowedRoles))
	}
}

// ListOrganizations lists all Organizations
// @Summary      List Organizations
// @Description  Lists all Organizations
// @Id 			 ListOrganizations
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.Organization
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations [get]
func (api *API) ListOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListOrganizations")
	defer span.End()
	var orgs []models.Organization

	db := api.db.WithContext(ctx)
	db = api.OrganizationIsReadableByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.Organization{}, c, "name")
	result := db.Find(&orgs)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// GetOrganizations gets a specific Organization
// @Summary      Get Organizations
// @Description  Gets a Organization by Organization ID
// @Id 			 GetOrganizations
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "Organization ID"
// @Success      200  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations/{id} [get]
func (api *API) GetOrganizations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetOrganizations",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var org models.Organization
	db := api.db.WithContext(ctx)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", k.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, org)
}

// DeleteOrganization handles deleting an existing organization and associated ipam prefix
// @Summary      Delete Organization
// @Description  Deletes an existing organization and associated IPAM prefix
// @Id 			 DeleteOrganization
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Organization ID"
// @Success      204  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations/{id} [delete]
func (api *API) DeleteOrganization(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteOrganization",
		trace.WithAttributes(
			attribute.String("organization", c.Param("id")),
			attribute.String("id", c.Param("id")),
		))
	defer span.End()

	if !api.FlagCheck(c, "multi-organization") {
		return
	}

	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var org models.Organization
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		result := api.OrganizationIsOwnedByCurrentUser(c, tx).
			First(&org, "id = ?", orgID)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("organization"))
			} else {
				return result.Error
			}
		}

		user := models.User{}
		result = tx.Unscoped().First(&user, "id = ?", orgID)
		if result.Error == nil {
			// found a user with the same id, it's a user's default org...
			return NewApiResponseError(http.StatusBadRequest, models.NewNotAllowedError("default organization cannot be deleted"))
		} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}

		return deleteOrganization(tx, orgID)
	})

	var apiResponseError *ApiResponseError
	if errors.As(err, &apiResponseError) {
		c.JSON(apiResponseError.Status, apiResponseError.Body)
	} else {
		api.SendInternalServerError(c, err)
	}

	c.JSON(http.StatusOK, org)
}

func deleteOrganization(tx *gorm.DB, orgID uuid.UUID) error {
	var count int64
	result := tx.Model(&models.Device{}).Where("organization_id = ?", orgID).Count(&count)
	if result.Error != nil {
		return result.Error
	}
	if count > 0 {
		return NewApiResponseError(http.StatusBadRequest, models.NewNotAllowedError("organization cannot be deleted while devices are still attached"))
	}

	// Cascade delete related records
	if res := tx.Where("organization_id = ?", orgID).Delete(&models.RegKey{}); res.Error != nil {
		return result.Error
	}
	if res := tx.Where("organization_id = ?", orgID).Delete(&models.SecurityGroup{}); res.Error != nil {
		return result.Error
	}
	if res := tx.Where("organization_id = ?", orgID).Delete(&models.VPC{}); res.Error != nil {
		return result.Error
	}
	type UserOrganization struct {
		UserID         uuid.UUID
		OrganizationID uuid.UUID
	}
	if res := tx.Where("organization_id = ?", orgID).Delete(&UserOrganization{}); res.Error != nil {
		return result.Error
	}
	if res := tx.Where("organization_id = ?", orgID).Delete(&models.Invitation{}); res.Error != nil {
		return result.Error
	}

	// Null out unique fields so that the org can be created later with the same values
	if res := tx.Model(&models.Organization{}).
		Where("id = ?", orgID).
		Updates(map[string]interface{}{
			"name":       nil,
			"deleted_at": gorm.DeletedAt{Time: time.Now(), Valid: true},
		}); res.Error != nil {
		return res.Error
	}

	return nil
}
