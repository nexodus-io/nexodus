package handlers

import (
	"errors"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/maps"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"gorm.io/gorm"
)

var allowedRoles = map[string]struct{}{
	"member": {},
	"owner":  {},
}

// CreateInvitation creates an invitation
// @Summary      Create an invitation
// @Description  Create an invitation to an organization
// @Id           CreateInvitation
// @Tags         Invitation
// @Accept       json
// @Produce      json
// @Param        Invitation  body     models.AddInvitation  true  "Add Invitation"
// @Success      201  {object}  models.Invitation
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/invitations [post]
func (api *API) CreateInvitation(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateInvitation")
	defer span.End()

	var request models.AddInvitation
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	if request.OrganizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("organization_id"))
		return
	}

	if request.UserID == nil && request.Email == nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("email or user_id"))
	}
	if request.UserID != nil && request.Email != nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("both email and user_id present"))
	}
	if len(util.FilterOutAllowed(request.Roles, allowedRoles)) > 0 {
		c.JSON(http.StatusBadRequest, models.NewFieldValidationError("roles", "allowed values are: "+strings.Join(maps.Keys(allowedRoles), ", ")))
		return
	}
	if len(request.Roles) == 0 {
		request.Roles = []string{"member"}
	}

	db := api.db.WithContext(ctx)

	// Only allow org owners to create invites...
	var org models.Organization
	if res := api.OrganizationIsOwnedByCurrentUser(c, db).
		First(&org, "id = ?", request.OrganizationID); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	// invitation expires after 1 week
	expiry := time.Now().Add(time.Hour * 24 * 7)
	invite := models.Invitation{
		OrganizationID: request.OrganizationID,
		ExpiresAt:      expiry,
		UserID:         request.UserID,
		Roles:          request.Roles,
	}

	var user models.User

	email := ""
	if request.Email != nil {
		// normalize the email address
		email = strings.TrimSpace(strings.ToLower(*request.Email))
		invite.Email = &email

		// check if user with the email already exists
		var uid models.UserIdentity
		if res := db.First(&uid, "kind = 'email' AND value = ?", *invite.Email); res.Error != nil {
			if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
				api.SendInternalServerError(c, res.Error)
				return
			}
			// it's fine if the user does not exist yet...
			invite.UserID = nil
		} else {
			invite.UserID = &uid.UserID
		}
	}

	// invite.UserID will be nil if the user doesn't exist
	if invite.UserID != nil {

		// load the user
		if res := db.
			Preload("Organizations").
			Preload("Invitations").
			First(&user, "id = ?", *invite.UserID); res.Error != nil {
			api.SendInternalServerError(c, res.Error)
			return
		}

		// check if user is already in the org
		for _, org := range user.Organizations {
			if org.ID == request.OrganizationID {
				c.JSON(http.StatusBadRequest, models.NewFieldValidationError("organization", "user is already in requested org"))
				return
			}
		}

		// check if user already has an invitation for the org
		for _, inv := range user.Invitations {
			if inv.OrganizationID == request.OrganizationID && inv.ExpiresAt.After(time.Now()) {
				c.JSON(http.StatusConflict, models.NewConflictsError(inv.ID.String()))
				return
			}
		}

		// See if the user has an associated email address
		if email == "" {
			var uid models.UserIdentity
			if res := db.First(&uid, "kind = 'email' AND user_id = ?", *invite.UserID); res.Error != nil {
				if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
					api.SendInternalServerError(c, res.Error)
					return
				}
			} else {
				email = uid.Value
				invite.Email = &email
			}
		}
	}

	from := models.User{}
	if res := db.First(&from, "id = ?", api.GetCurrentUserID(c)); res.Error != nil {
		api.SendInternalServerError(c, res.Error)
		return
	}
	invite.FromID = from.ID

	if res := db.Create(&invite); res.Error != nil {
		api.SendInternalServerError(c, res.Error)
		return
	}

	if email != "" {
		err := api.sendInvitationEmail(from.UserName, email, &invite, org.Name)
		if err != nil { // invite will still be created even if email fails, so don't return an error
			api.Logger(c).Warn("error sending invitation email", "error", err)
		}
	}

	c.JSON(http.StatusCreated, invite)
}

// ListInvitations lists invitations
// @Summary      List Invitations
// @Description  Lists all invitations
// @Id           ListInvitations
// @Tags         Invitation
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.Invitation
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/invitations [get]
func (api *API) ListInvitations(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListInvitations")
	defer span.End()
	invitations := make([]*models.Invitation, 0)
	db := api.db.WithContext(ctx)
	db = api.InvitationIsForCurrentUserOrOrgOwner(c, db)
	db = FilterAndPaginate(db, &models.Invitation{}, c, "id")
	result := db.
		Joins("From").
		Joins("Organization").
		Find(&invitations)
	if result.Error != nil {
		api.SendInternalServerError(c, errors.New("error fetching keys from db"))
		return
	}
	c.JSON(http.StatusOK, invitations)
}

func (api *API) InvitationIsForCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	return db.Where("user_id = ?", userId)
}

func (api *API) InvitationIsForCurrentUserOrOrgOwner(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	return db.Where(api.CurrentUserHasRole(c, db, "organization_id", OwnerRoles).
		Or(db.Where("user_id = ?", userId)))
}

// GetInvitation gets a specific Invitation
// @Summary      Get Invitation
// @Description  Gets an Invitation by Invitation ID
// @Id 			 GetInvitation
// @Tags         Invitation
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "Invitation ID"
// @Success      200  {object}  models.Invitation
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/invitations/{id} [get]
func (api *API) GetInvitation(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetInvitation",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var org models.Invitation
	db := api.db.WithContext(ctx)
	result := api.InvitationIsForCurrentUserOrOrgOwner(c, db).
		Joins("From").
		Joins("Organization").
		// we have to qualify the column name here because of the join
		First(&org, "invitations.id = ?", id.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("invitation"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	c.JSON(http.StatusOK, org)
}

// AcceptInvitation accepts an invitation
// @Summary      Accept an invitation
// @Description  Accept an invitation to an organization
// @Id           AcceptInvitation
// @Tags         Invitation
// @Accept		 json
// @Produce      json
// @Param        id   path      string  true "Invitation ID"
// @Success      204
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/invitations/{id}/accept [post]
func (api *API) AcceptInvitation(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "AcceptInvitation",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var invitation models.Invitation
	err = api.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if res := api.InvitationIsForCurrentUser(c, tx).
			First(&invitation, "id = ?", id); res.Error != nil {
			return errInvitationNotFound
		}

		var user models.User
		if res := tx.First(&user, "id = ?", invitation.UserID); res.Error != nil {
			return errUserNotFound
		}

		userOrganization := models.UserOrganization{
			UserID:         user.ID,
			OrganizationID: invitation.OrganizationID,
			Roles:          invitation.Roles,
		}
		if res := tx.Create(&userOrganization); res.Error != nil {
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
			api.SendInternalServerError(c, err)
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
// @Accept		 json
// @Produce      json
// @Param        id   path      string  true "Invitation ID"
// @Success      204  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/invitations/{id} [delete]
func (api *API) DeleteInvitation(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteInvitation",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()

	//multiOrganizationEnabled, err := api.fflags.GetFlag("multi-organization")
	//if err != nil {
	//	api.SendInternalServerError(c, err)
	//	return
	//}
	//allowForTests := c.GetString("_apex.testCreateOrganization")
	//if !multiOrganizationEnabled && allowForTests != "true" {
	//	c.JSON(http.StatusMethodNotAllowed, models.NewNotAllowedError("multi-organization support is disabled"))
	//	return
	//}

	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	var invitation models.Invitation
	db := api.db.WithContext(ctx)
	if res := api.InvitationIsForCurrentUserOrOrgOwner(c, db).
		First(&invitation, "id = ?", k); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("invitation"))
		return
	}

	if res := db.Delete(&models.Invitation{}, k); res.Error != nil {
		api.SendInternalServerError(c, res.Error)
		return
	}
	c.Status(http.StatusNoContent)
}
