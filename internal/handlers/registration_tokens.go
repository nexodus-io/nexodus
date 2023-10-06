package handlers

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
	"net/http"
	"time"
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
	if request.OrganizationID == noUUID {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("organization_id"))
		return
	}

	// The org has to be owned by the user.
	var org models.Organization
	db := api.db.WithContext(ctx)
	if res := api.OrganizationIsOwnedByCurrentUser(c, db).
		First(&org, "id = ?", request.OrganizationID); res.Error != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("organization"))
		return
	}

	if request.OrganizationID == noUUID {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("organization_id"))
		return
	}

	userId := c.Value(gin.AuthUserKey).(string)

	tokenId := uuid.New()

	claims := models.NexodusClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  api.URL,
			ID:      tokenId.String(),
			Subject: userId,
		},
		OrganizationID: org.ID,
		Scope:          "reg-token",
	}
	if request.Expiration != nil {
		claims.ExpiresAt = jwt.NewNumericDate(*request.Expiration)
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(api.PrivateKey)
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}

	// Let store the reg token... without the client id yet... to avoid creating
	// clients in KC that are not correlated with our DB.
	record := models.RegistrationTokenRecord{
		Base: models.Base{
			ID: tokenId,
		},
		UserID:         userId,
		Description:    request.Description,
		OrganizationID: request.OrganizationID,
		BearerToken:    token,
	}
	if res := db.Create(&record); res.Error != nil {
		api.sendInternalServerError(c, res.Error)
		return
	}

	result, err := api.asRegistrationToken(record)
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

func NxodusClaims(c *gin.Context, tx *gorm.DB) (*models.NexodusClaims, *ApiResponseError) {

	claims := models.NexodusClaims{}
	err := util.JsonUnmarshal(c.GetStringMap("_nexodus.Claims"), &claims)
	if err != nil {
		return nil, NewApiResponseError(http.StatusUnauthorized, models.BaseError{
			Error: "invalid registration token",
		})
	}

	switch claims.Scope {
	case "reg-token":

		record := models.RegistrationTokenRecord{}
		err = tx.First(&record, "id=?", claims.ID).Error
		if err != nil {
			return nil, NewApiResponseError(http.StatusUnauthorized, models.BaseError{
				Error: "invalid registration token",
			})
		}

	case "device-token":

		record := models.Device{}
		err = tx.First(&record, "id=?", claims.ID).Error
		if err != nil {
			return nil, NewApiResponseError(http.StatusUnauthorized, models.BaseError{
				Error: "invalid device token",
			})
		}

	default:
		return nil, nil
	}

	return &claims, nil
}

func (api *API) asRegistrationToken(record models.RegistrationTokenRecord) (models.RegistrationToken, error) {
	claims := models.NexodusClaims{}
	tkn, err := jwt.ParseWithClaims(record.BearerToken, &claims, func(token *jwt.Token) (any, error) {
		return &api.PrivateKey.PublicKey, nil
	})
	if err != nil {
		return models.RegistrationToken{}, err
	}
	if !tkn.Valid {
		return models.RegistrationToken{}, errors.New("registration token is not valid")
	}
	if record.OrganizationID != claims.OrganizationID {
		return models.RegistrationToken{}, errors.New("registration token is not valid, org id mismatch")
	}

	var expiration *time.Time
	if claims.ExpiresAt != nil {
		expiration = &claims.ExpiresAt.Time
	}

	deviceId := &claims.DeviceID
	if claims.DeviceID == uuid.Nil {
		deviceId = nil
	}

	return models.RegistrationToken{
		Base:           record.Base,
		UserID:         record.UserID,
		OrganizationID: record.OrganizationID,
		BearerToken:    record.BearerToken,
		Description:    record.Description,
		Expiration:     expiration,
		DeviceID:       deviceId,
	}, nil
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
	records := []models.RegistrationTokenRecord{}
	db := api.db.WithContext(ctx)
	db = api.RegistrationTokenIsForCurrentUserOrOrgOwner(c, db)
	db = FilterAndPaginate(db, &models.RegistrationTokenRecord{}, c, "id")
	result := db.Find(&records)
	if result.Error != nil {
		api.sendInternalServerError(c, fmt.Errorf("error fetching keys from db: %w", result.Error))
		return
	}
	results := []models.RegistrationToken{}
	for _, record := range records {
		token, err := api.asRegistrationToken(record)
		if err != nil {
			api.sendInternalServerError(c, err)
			return
		}
		results = append(results, token)
	}
	c.JSON(http.StatusOK, results)
}

// GetRegistrationToken gets a specific RegistrationToken
// @Summary      Get a RegistrationToken
// @Description  Gets a RegistrationToken by RegistrationToken ID
// @Id 			 GetRegistrationToken
// @Tags         RegistrationToken
// @Accept       json
// @Produce      json
// @Param		 token-id   path      string true "RegistrationToken ID"
// @Success      200  {object}  models.Organization
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/registration-tokens/{token-id} [get]
func (api *API) GetRegistrationToken(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetRegistrationToken",
		trace.WithAttributes(
			attribute.String("token-id", c.Param("token-id")),
		))
	defer span.End()
	id, err := uuid.Parse(c.Param("token-id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("token-id"))
		return
	}

	var record models.RegistrationTokenRecord
	db := api.db.WithContext(ctx)
	db = api.RegistrationTokenIsForCurrentUserOrOrgOwner(c, db)
	result := db.First(&record, "id = ?", id.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("registration token"))
		} else {
			api.sendInternalServerError(c, result.Error)
		}
		return
	}

	token, err := api.asRegistrationToken(record)
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, token)
}

func (api *API) RegistrationTokenIsForCurrentUser(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		userId := c.Value(gin.AuthUserKey).(string)

		// this could potentially be driven by rego output
		return db.Where("user_id = ?", userId)
	}
}

func (api *API) RegistrationTokenIsForCurrentUserOrOrgOwner(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := c.Value(gin.AuthUserKey).(string)

	// this could potentially be driven by rego output
	if api.dialect == database.DialectSqlLite {
		return db.Where("user_id = ? OR organization_id in (SELECT id FROM organizations where owner_id=?)", userId, userId)
	} else {
		return db.Where("user_id = ? OR organization_id::text in (SELECT id::text FROM organizations where owner_id=?)", userId, userId)
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

	var record models.RegistrationTokenRecord
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

	token, err := api.asRegistrationToken(record)
	if err != nil {
		api.sendInternalServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, token)
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
			Algorithm: "RS256",
			Use:       "sig",
			Key:       &api.PrivateKey.PublicKey,
		}},
	})
}
