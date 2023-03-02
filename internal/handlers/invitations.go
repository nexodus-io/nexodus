package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"gorm.io/gorm"
)

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
	var user models.User
	if res := api.db.WithContext(ctx).Preload("Organizations").Preload("Invitations").First(&user, "id = ?", request.UserID); res.Error != nil {
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
		if res := tx.First(&invitation, "id = ?", k); res.Error != nil {
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
	if res := api.db.WithContext(ctx).Delete(&models.Invitation{}, k); res.Error != nil {
		c.JSON(http.StatusInternalServerError, models.NewApiInternalError(res.Error))
		return
	}
	c.Status(http.StatusNoContent)
}
