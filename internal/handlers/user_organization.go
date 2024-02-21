package handlers

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"net/http"
)

// IsMemberOfOrg checks if the current user is a member of the organization, returns true if he is.
func (api *API) IsMemberOfOrg(c *gin.Context, orgId uuid.UUID) (bool, error) {
	var org models.Organization
	db := api.db.WithContext(c)
	result := api.OrganizationIsReadableByCurrentUser(c, db).
		First(&org, "id = ?", orgId.String())
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return false, nil
		} else {
			return false, result.Error
		}
	}
	return true, nil
}

// IsOwnerOfOrg checks if the current user is a owner of the organization, returns true if he is.
func (api *API) IsOwnerOfOrg(c *gin.Context, orgId uuid.UUID) (bool, error) {
	var org models.Organization
	db := api.db.WithContext(c)
	result := api.OrganizationIsOwnedByCurrentUser(c, db).
		First(&org, "id = ?", orgId.String())
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return false, nil
		} else {
			return false, result.Error
		}
	}
	return true, nil
}

// ListOrganizationUsers lists the users of an organization
// @Summary      List Organization Users
// @Description  Lists all the users of an organization
// @Id 			 ListOrganizationUsers
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "Organization ID"
// @Success      200  {object}  []models.UserOrganization
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations/{id}/users [get]
func (api *API) ListOrganizationUsers(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListOrganizationUsers",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))

	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	member, err := api.IsMemberOfOrg(c, id)
	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}
	if !member {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	db := api.db.WithContext(ctx)
	db = db.Joins("User")
	db = db.Where("organization_id = ?", id)
	db = FilterAndPaginate(db, &models.UserOrganization{}, c, `"User".full_name`)

	var orgs []models.UserOrganization
	result := db.Find(&orgs)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		api.SendInternalServerError(c, result.Error)
		return
	}

	c.JSON(http.StatusOK, orgs)
}

// GetOrganizationUser gets a specific Organization User
// @Summary      Get Organization User
// @Description  Gets a Organization User by Organization ID and User ID
// @Id 			 GetOrganizationUser
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "Organization ID"
// @Param		 uid  path      string true "User ID"
// @Success      200  {object}  models.UserOrganization
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations/{id}/users/{uid} [get]
func (api *API) GetOrganizationUser(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetOrganizationUser",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
			attribute.String("uid", c.Param("uid")),
		))
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("uid"))
		return
	}

	member, err := api.IsMemberOfOrg(c, id)
	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}
	if !member {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	db := api.db.WithContext(ctx)
	db = db.Joins("User")
	db = db.Where("organization_id=? AND user_id=?", id, uid)
	var model models.UserOrganization
	result := db.First(&model)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("user_organization"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, model)
}

// DeleteOrganizationUser handles deleting a user from an organization
// @Summary      Delete a Organization User
// @Description  Deletes an existing organization user
// @Id 			 DeleteOrganizationUser
// @Tags         Organizations
// @Accept       json
// @Produce      json
// @Param        id   path      string true "Organization ID"
// @Param		 uid  path      string true "User ID"
// @Success      204  {object}  models.UserOrganization
// @Failure      400  {object}  models.ValidationError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/organizations/{id}/users/{uid} [delete]
func (api *API) DeleteOrganizationUser(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteOrganization",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
			attribute.String("uid", c.Param("uid")),
		))
	defer span.End()

	if !api.FlagCheck(c, "multi-organization") {
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("uid"))
		return
	}

	var model models.UserOrganization
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		isOwner, err := api.IsOwnerOfOrg(c, id)
		if err != nil {
			return err
		}
		if !isOwner {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("organization"))
		}

		// don't allow deleting the org owner...
		userId := api.GetCurrentUserID(c)
		if uid == userId {
			return NewApiResponseError(http.StatusBadRequest, models.NewBadPathParameterErrorAndReason("uid", "cannot delete owner of the organization"))
		}

		result := tx.Joins("User").
			Where("organization_id=? AND user_id=?", id, uid).
			First(&model)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("user"))
			}
			return err
		}
		result = tx.Delete(&model)
		if result.Error != nil {
			return result.Error
		}

		result = tx.Where("organization_id=? AND owner_id=?", id, uid).
			Delete(&models.RegKey{})
		if result.Error != nil {
			return result.Error
		}
		result = tx.Where("organization_id=? AND owner_id=?", id, uid).
			Delete(&models.Device{})
		if result.Error != nil {
			return result.Error
		}
		result = tx.Where("organization_id=? AND owner_id=?", id, uid).
			Delete(&models.Site{})
		if result.Error != nil {
			return result.Error
		}

		return nil
	})
	if err != nil {
		var apiResponseError *ApiResponseError
		if errors.As(err, &apiResponseError) {
			c.JSON(apiResponseError.Status, apiResponseError.Body)
			return
		} else {
			api.SendInternalServerError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, model)
}
