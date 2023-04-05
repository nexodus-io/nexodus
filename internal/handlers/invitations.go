package handlers

import (
	"errors"
	"github.com/nexodus-io/nexodus/internal/database"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"gorm.io/gorm"
)

// CreateInvitation creates an invitation
// @Summary      Create an invitation
// @Description  Create an invitation to an organization
// @Id           CreateInvitation
// @Tags         Invitation
// @Accepts		 json
// @Produce      json
// @Param        Invitation  body     models.AddInvitation  true  "Add Invitation"
// @Success      201  {object}  models.Invitation
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/invitations [post]
func (api *API) CreateInvitation(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "InviteUserToOrganization")
	defer span.End()
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}
	allowForTests := c.GetString("_apex.testCreateOrganization")
	if !multiOrganizationEnabled && allowForTests != "true" {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
		return
	}
	var request models.AddInvitation
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	// Only allow org owners to create invites...
	var org models.Organization
	if res := api.db.WithContext(ctx).
		Scopes(api.OrganizationIsOwnedByCurrentUser(c)).
		First(&org, "id = ?", request.OrganizationID); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	var user models.User
	if res := api.db.WithContext(ctx).
		Preload("Organizations").
		Preload("Invitations").
		First(&user, "id = ?", request.UserID); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("user"))
		return
	}

	for _, org := range user.Organizations {
		if org.ID == request.OrganizationID {
			c.JSON(http.StatusBadRequest, models.NewFieldValidationError("organization", "user is already in requested org"))
			return
		}
	}
	for _, inv := range user.Invitations {
		if inv.OrganizationID == request.OrganizationID && inv.Expiry.After(time.Now()) {
			c.JSON(http.StatusConflict, models.NewConflictsError(inv.ID.String()))
			return
		}
	}

	invite := models.NewInvitation(user.ID, request.OrganizationID)
	if res := api.db.WithContext(ctx).Create(&invite); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}
	c.JSON(http.StatusCreated, invite)
}

// ListInvitations lists invitations
// @Summary      List Invitations
// @Description  Lists all invitations
// @Id           ListInvitations
// @Tags         Invitation
// @Accepts		 json
// @Produce      json
// @Success      200  {object}  []models.Invitation
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/invitations [get]
func (api *API) ListInvitations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListInvitations")
	defer span.End()
	users := make([]*models.Invitation, 0)
	result := api.db.WithContext(ctx).
		Scopes(api.InvitationIsForCurrentUserOrOrgOwner(c)).
		Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "error fetching keys from db"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (api *API) InvitationIsForCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)

		// this could potentially be driven by rego output
		return db.Where("user_id = ?", userId)
	}
}

func (api *API) InvitationIsForCurrentUserOrOrgOwner(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)

		// this could potentially be driven by rego output
		if api.dialect == database.DialectSqlLite {
			return db.Where("user_id = ? OR organization_id in (SELECT id FROM organizations where owner_id=?)", userId, userId)
		} else {
			return db.Where("user_id = ? OR organization_id::text in (SELECT id::text FROM organizations where owner_id=?)", userId, userId)
		}
	}
}

// AcceptInvitation accepts an invitation
// @Summary      Accept an invitation
// @Description  Accept an invitation to an organization
// @Id           AcceptInvitation
// @Tags         Invitation
// @Accepts		 json
// @Produce      json
// @Param        invitation   path      string  true "Invitation ID"
// @Success      204
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /api/invitations/:invitation/accept [post]
func (api *API) AcceptInvitation(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "InviteUserToOrganization")
	defer span.End()
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}
	allowForTests := c.GetString("_apex.testCreateOrganization")
	if !multiOrganizationEnabled && allowForTests != "true" {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
		return
	}
	k, err := uuid.Parse(c.Param("invitation"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("invitation"))
		return
	}

	var invitation models.Invitation
	err = api.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if res := tx.
			Scopes(api.InvitationIsForCurrentUser(c)).
			First(&invitation, "id = ?", k); res.Error != nil {
			return errInvitationNotFound
		}
		var user models.User
		if res := tx.Preload("Organizations").First(&user, "id = ?", invitation.UserID); res.Error != nil {
			return errUserNotFound
		}

		var org models.Organization
		if res := tx.First(&org, "id = ?", invitation.OrganizationID); res.Error != nil {
			return errOrgNotFound
		}
		user.Organizations = append(user.Organizations, &org)
		if res := tx.Save(&user); res.Error != nil {
			return res.Error
		}
		if res := tx.Delete(&invitation); res.Error != nil {
			return res.Error
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, errInvitationNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("invitation"))
		} else if errors.Is(err, errUserNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("user"))
		} else if errors.Is(err, errOrgNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		} else {
			c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteInvitation handles deleting an existing organization and associated ipam prefix
// @Summary      Delete Invitation
// @Description  Deletes an existing invitation
// @Id 			 DeleteInvitation
// @Tags         Invitation
// @Accepts		 json
// @Produce      json
// @Param        invitation   path      string  true "Invitation ID"
// @Success      204  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.BaseError
// @Router       /api/invitations/{invitation} [delete]
func (api *API) DeleteInvitation(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteInvitation")
	defer span.End()
	multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(err))
		return
	}
	allowForTests := c.GetString("_apex.testCreateOrganization")
	if !multiOrganizationEnabled && allowForTests != "true" {
		c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
		return
	}
	k, err := uuid.Parse(c.Param("invitation"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("invitation"))
		return
	}

	var invitation models.Invitation
	if res := api.db.WithContext(ctx).
		Scopes(api.InvitationIsForCurrentUserOrOrgOwner(c)).
		First(&invitation, "id = ?", k); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("invitation"))
		return
	}

	if res := api.db.WithContext(ctx).Delete(&models.Invitation{}, k); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(res.Error))
		return
	}
	c.Status(http.StatusNoContent)
}
