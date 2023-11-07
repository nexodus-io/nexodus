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

// CreateRegKey creates a RegKey
// @Summary      Create a RegKey
// @Description  Create a RegKey for a vpc
// @Id           CreateRegKey
// @Tags         RegKey
// @Accept       json
// @Produce      json
// @Param        RegKey  body     models.AddRegKey  true  "Add RegKey"
// @Success      201  {object}  models.RegKey
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/reg-keys [post]
func (api *API) CreateRegKey(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "CreateRegKey")
	defer span.End()

	var request models.AddRegKey
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	// org field is required...
	if request.VpcID == uuid.Nil {
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

	if request.VpcID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("vpc_id"))
		return
	}

	userId := api.GetCurrentUserID(c)

	// lets use a wg private key as the token, since it should be hard to guess.
	token, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		api.SendInternalServerError(c, err)
		return
	}

	// Let store the reg token... without the client id yet... to avoid creating
	// clients in KC that are not correlated with our DB.
	record := models.RegKey{
		OwnerID:        userId,
		VpcID:          vpc.ID,
		OrganizationID: vpc.OrganizationID,
		BearerToken:    "RT:" + token.String(),
		Description:    request.Description,
		ExpiresAt:      request.ExpiresAt,
	}
	if request.SingleUse {
		deviceID := uuid.New()
		record.DeviceId = &deviceID
	}

	if res := db.Create(&record); res.Error != nil {
		api.SendInternalServerError(c, res.Error)
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

// ListRegKeys lists reg keys
// @Summary      List reg keys
// @Description  Lists all reg keys
// @Id           ListRegKeys
// @Tags         RegKey
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.RegKey
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/reg-keys [get]
func (api *API) ListRegKeys(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListRegKeys")
	defer span.End()
	records := []models.RegKey{}
	db := api.db.WithContext(ctx)
	db = api.RegKeyIsForCurrentUserOrOrgOwner(c, db)
	db = FilterAndPaginate(db, &models.RegKey{}, c, "id")
	result := db.Find(&records)
	if result.Error != nil {
		api.SendInternalServerError(c, fmt.Errorf("error fetching keys from db: %w", result.Error))
		return
	}
	c.JSON(http.StatusOK, records)
}

// GetRegKey gets a specific RegKey
// @Summary      Get a RegKey
// @Description  Gets a RegKey by RegKey ID
// @Id 			 GetRegKey
// @Tags         RegKey
// @Accept       json
// @Produce      json
// @Param		 id   path      string true "RegKey ID"
// @Success      200  {object}  models.RegKey
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/reg-keys/{id} [get]
func (api *API) GetRegKey(c *gin.Context) {
	tokenId := c.Param("id")
	ctx, span := tracer.Start(c.Request.Context(), "GetRegKey",
		trace.WithAttributes(
			attribute.String("id", tokenId),
		))
	defer span.End()

	tokenClaims, apierr := NxodusClaims(c, api.db.WithContext(ctx))
	if apierr != nil {
		c.JSON(apierr.Status, apierr.Body)
		return
	}

	var record models.RegKey
	db := api.db.WithContext(ctx)
	db = api.RegKeyIsForCurrentUserOrOrgOwner(c, db)

	if tokenClaims != nil && tokenClaims.Scope == "reg-token" {
		db = db.Where("id = ?", tokenClaims.ID)
		if tokenId != "me" {
			c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("key-id"))
		}
	} else {
		id, err := uuid.Parse(tokenId)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("key-id"))
			return
		}
		db = db.Where("id = ?", id.String())
	}

	result := db.First(&record)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("reg key"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}
	c.JSON(http.StatusOK, record)
}

func (api *API) RegKeyIsForCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)

	// this could potentially be driven by rego output
	return db.Where("owner_id = ?", userId)
}

func (api *API) RegKeyIsForCurrentUserOrOrgOwner(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)

	// this could potentially be driven by rego output
	if api.dialect == database.DialectSqlLite {
		return db.Where("owner_id = ? OR organization_id in (SELECT id FROM organizations where owner_id=?)", userId, userId)
	} else {
		return db.Where("owner_id = ? OR organization_id::text in (SELECT id::text FROM organizations where owner_id=?)", userId, userId)
	}
}

// DeleteRegKey handles deleting a RegKey
// @Summary      Delete RegKey
// @Description  Deletes an existing RegKey
// @Id 			 DeleteRegKey
// @Tags         RegKey
// @Accept		 json
// @Produce      json
// @Param		 id   path      string true "RegKey ID"
// @Success      204  {object}  models.RegKey
// @Failure      400  {object}  models.BaseError
// @Failure      405  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/reg-keys/{id} [delete]
func (api *API) DeleteRegKey(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteRegKey",
		trace.WithAttributes(
			attribute.String("id", c.Param("id")),
		))
	defer span.End()

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("key-id"))
		return
	}

	var record models.RegKey
	err = api.transaction(ctx, func(tx *gorm.DB) error {
		res := api.RegKeyIsForCurrentUser(c, tx).
			First(&record, "id = ?", id)
		if res.Error != nil {
			return res.Error
		}

		res = tx.Delete(&models.RegKey{}, id)
		if res.Error != nil {
			return res.Error
		}
		return nil
	})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("reg key"))
		return
	} else if err != nil {
		api.SendInternalServerError(c, err)
		return
	}

	c.JSON(http.StatusOK, record)
}

// Certs gets the jwks that can be used to verify JWTs created by this server.
// @Summary      gets the jwks
// @Description  gets the jwks that can be used to verify JWTs created by this server.
// @Id           Certs
// @Tags         RegKey
// @Accept		 json
// @Produce      json
// @Success      200  {object} interface{}
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /device/certs [get]
func (api *API) Certs(c *gin.Context) {
	data, err := api.JSONWebKeySet()
	if err != nil {
		api.SendInternalServerError(c, err)
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
