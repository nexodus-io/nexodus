package handlers

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v3"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
	"net/http"
)

// CreateRegistrationToken creates a RegistrationToken
// @Summary      Create a RegistrationToken
// @Description  Create a RegistrationToken to an organization
// @Id           CreateRegistrationToken
// @Tags         RegistrationToken
// @Accept       json
// @Produce      json
// @Param        RegistrationToken  body     models.AddRegistrationToken  true  "Add RegistrationToken"
// @Success      201  {object}  models.RegistrationToken
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/registration-tokens [post]
func (api *API) CreateRegistrationToken(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateRegistrationToken")
	defer span.End()

	var request models.AddRegistrationToken
	if err := c.BindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError())
		return
	}

	// org field is required...
	if request.VpcID == noUUID {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("vpc_id"))
		return
	}

	// The vpc has to be owned by the user.
	var vpc models.VPC
	db := api.db.WithContext(ctx)
	if res := api.VPCIsOwnedByCurrentUser(c, db).
		First(&vpc, "id = ?", request.VpcID.String()); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("vpc"))
		return
	}

	if request.VpcID == noUUID {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("vpc_id"))
		return
	}

	userId := c.Value(gin.AuthUserKey).(string)

	// lets use a wg private key as the token, since it should be hard to guess.
	token, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}

	// Let store the reg token... without the client id yet... to avoid creating
	// clients in KC that are not correlated with our DB.
	record := models.RegistrationToken{
		OwnerID:        userId,
		VpcID:          vpc.ID,
		OrganizationID: vpc.OrganizationID,
		BearerToken:    "RT:" + token.String(),
		Description:    request.Description,
		Expiration:     request.Expiration,
	}
	if request.SingleUse {
		deviceID := uuid.New()
		record.DeviceId = &deviceID
	}

	if res := db.Create(&record); res.Error != nil {
		api.sendInternalServerError(c, res.Error)
		return
	}
	c.JSON(http.StatusCreated, record)
}

func NxodusClaims(c *gin.Context, tx *gorm.DB) (*models.NexodusClaims, *ApiResponseError) {
	claims := models.NexodusClaims{}
	err := util.JsonUnmarshal(c.GetStringMap("_nexodus.Claims"), &claims)
	if err != nil {
		return nil, NewApiResponseError(http.StatusUnauthorized, models.BaseError{
			Error: "invalid authorization token",
		})
	}
	return &claims, nil
}

// ListRegistrationTokens lists registration tokens
// @Summary      List RegistrationTokens
// @Description  Lists all registration tokens
// @Id           ListRegistrationTokens
// @Tags         RegistrationToken
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.RegistrationToken
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/registration-tokens [get]
func (api *API) ListRegistrationTokens(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListRegistrationTokens")
	defer span.End()
	records := []models.RegistrationToken{}
	db := api.db.WithContext(ctx)
	db = api.RegistrationTokenIsForCurrentUserOrOrgOwner(c, db)
	db = FilterAndPaginate(db, &models.RegistrationToken{}, c, "id")
	result := db.Find(&records)
	if result.Error != nil {
		api.sendInternalServerError(c, fmt.Errorf("error fetching keys from db: %w", result.Error))
		return
	}
	c.JSON(http.StatusOK, records)
}

// GetRegistrationToken gets a specific RegistrationToken
// @Summary      Get a RegistrationToken
// @Description  Gets a RegistrationToken by RegistrationToken ID
// @Id 			 GetRegistrationToken
// @Tags         RegistrationToken
// @Accept       json
// @Produce      json
// @Param		 token-id   path      string true "RegistrationToken ID"
// @Success      200  {object}  models.RegistrationToken
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/registration-tokens/{token-id} [get]
func (api *API) GetRegistrationToken(c *gin.Context) {
	tokenId := c.Param("token-id")
	ctx, span := tracer.Start(c.Request.Context(), "GetRegistrationToken",
		trace.WithAttributes(
			attribute.String("token-id", tokenId),
		))
	defer span.End()

	tokenClaims, apierr := NxodusClaims(c, api.db.WithContext(ctx))
	if apierr != nil {
		c.JSON(apierr.Status, apierr.Body)
		return
	}

	var record models.RegistrationToken
	db := api.db.WithContext(ctx)
	db = api.RegistrationTokenIsForCurrentUserOrOrgOwner(c, db)

	if tokenClaims != nil && tokenClaims.Scope == "reg-token" {
		db = db.Where("id = ?", tokenClaims.ID)
		if tokenId != "me" {
			c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("token-id"))
		}
	} else {
		id, err := uuid.Parse(tokenId)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("token-id"))
			return
		}
		db = db.Where("id = ?", id.String())
	}

	result := db.First(&record)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("registration token"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}
	c.JSON(http.StatusOK, record)
}

func (api *API) RegistrationTokenIsForCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)

		// this could potentially be driven by rego output
		return db.Where("owner_id = ?", userId)
	}
}

func (api *API) RegistrationTokenIsForCurrentUserOrOrgOwner(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := c.Value(gin.AuthUserKey).(string)

	// this could potentially be driven by rego output
	if api.dialect == database.DialectSqlLite {
		return db.Where("owner_id = ? OR organization_id in (SELECT id FROM organizations where owner_id=?)", userId, userId)
	} else {
		return db.Where("owner_id = ? OR organization_id::text in (SELECT id::text FROM organizations where owner_id=?)", userId, userId)
	}
}

// DeleteRegistrationToken handles deleting a RegistrationToken
// @Summary      Delete RegistrationToken
// @Description  Deletes an existing RegistrationToken
// @Id 			 DeleteRegistrationToken
// @Tags         RegistrationToken
// @Accept		 json
// @Produce      json
// @Param		 token-id   path      string true "RegistrationToken ID"
// @Success      204  {object}  models.RegistrationToken
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/registration-tokens/{token-id} [delete]
func (api *API) DeleteRegistrationToken(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteRegistrationToken")
	defer span.End()

	id, err := uuid.Parse(c.Param("token-id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("token-id"))
		return
	}

	var record models.RegistrationToken
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		res := api.RegistrationTokenIsForCurrentUserOrOrgOwner(c, tx).
			First(&record, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Delete(&models.RegistrationToken{}, id)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("registration token"))
		return
	} else if err != nil {
		api.sendInternalServerError(c, err)
		return
	}

	c.JSON(http.StatusOK, record)
}

// Certs gets the jwks that can be used to verify JWTs created by this server.
// @Summary      gets the jwks
// @Description  gets the jwks that can be used to verify JWTs created by this server.
// @Id           Certs
// @Tags         RegistrationToken
// @Accept		 json
// @Produce      json
// @Success      200  {object} interface{}
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /device/certs [get]
func (api *API) Certs(c *gin.Context) {
	data, err := api.JSONWebKeySet()
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}
	c.Data(200, "application/json", data)
}

func (api *API) JSONWebKeySet() ([]byte, error) {
	return json.Marshal(jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{{
			Algorithm:    "RS256",
			Use:          "sig",
			Key:          &api.PrivateKey.PublicKey,
			Certificates: api.Certificates,
		}},
	})
}
