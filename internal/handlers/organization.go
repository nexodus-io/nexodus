package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}
	allowForTests := c.GetString("nexodus.testCreateOrganization")
	if (!multiOrganizationEnabled && allowForTests != "true") || allowForTests == "false" {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
		return
	}
	userId := api.GetCurrentUserID(c)

	var request models.AddOrganization
	// Call ShouldBindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	if request.Name == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("name"))
		return
	}

	var org models.Organization
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		var user models.User
		if res := tx.First(&user, "id = ?", userId); res.Error != nil {
			return errUserNotFound
		}

		org = models.Organization{
			Name:        request.Name,
			OwnerID:     userId,
			Description: request.Description,
		}

		if res := tx.Create(&org); res.Error != nil {
			if database.IsDuplicateError(res.Error) {
				return errDuplicateOrganization{ID: org.ID.String()}
			}
			api.logger.Error("Failed to create organization: ", res.Error)
			return res.Error
		}

		// Create a default security group for the organization
		sg, err := api.createDefaultSecurityGroup(ctx, tx, org.ID)
		if err != nil {
			api.logger.Error("Failed to create default security group for organization: ", err)
			return err
		}

		// Update the org with the new security group id
		if err := api.updateOrganizationSecGroupId(ctx, tx, sg.ID, org.ID); err != nil {
			return fmt.Errorf("failed to update the default security group with an org id: %w", err)
		}
		org.SecurityGroupId = sg.ID

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

	// this could potentially be driven by rego output
	if api.dialect == database.DialectSqlLite {
		return db.Where("owner_id = ? OR id in (SELECT organization_id FROM user_organizations where user_id=?)", userId, userId)
	} else {
		return db.Where("owner_id = ? OR id::text in (SELECT organization_id::text FROM user_organizations where user_id=?)", userId, userId)
	}
}

func (api *API) OrganizationIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	// this could potentially be driven by rego output
	return db.Where("owner_id = ?", userId)
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
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}
	if !multiOrganizationEnabled {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
		return
	}

	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("organization"))
		return
	}

	var org models.Organization
	db := api.db.WithContext(ctx)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", orgID)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	type userOrgMapping struct {
		UserID         string
		OrganizationID uuid.UUID
	}
	var usersInOrg []userOrgMapping
	if res := db.Table("user_organizations").Select("user_id", "organization_id").Where("organization_id = ?", org.ID).Scan(&usersInOrg); res.Error != nil {
		api.SendInternalServerError(c, res.Error)
		return
	}

	if res := api.db.Select(clause.Associations).Delete(&org); res.Error != nil {
		api.SendInternalServerError(c, fmt.Errorf("failed to delete the organization: %w", err))
		return
	}

	c.JSON(http.StatusOK, org)
}
