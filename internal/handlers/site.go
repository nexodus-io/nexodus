package handlers

import (
	"errors"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/wgcrypto"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type siteList []*models.Site

func (d siteList) Item(i int) (any, string, uint64, gorm.DeletedAt) {
	item := d[i]
	return item, item.ID.String(), item.Revision, item.DeletedAt
}

func (d siteList) Len() int {
	return len(d)
}

// ListSites lists all sites
// @Summary      List Sites
// @Description  Lists all sites
// @Id  		 ListSites
// @Tags         Sites
// @Accept       json
// @Produce      json
// @Success      200  {object}  []models.Site
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/sites [get]
func (api *API) ListSites(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "ListSites")
	defer span.End()

	if !api.FlagCheck(c, "sites") {
		return
	}

	sites := make([]models.Site, 0)

	db := api.db.WithContext(ctx)
	db = api.SiteIsOwnedByCurrentUser(c, db)
	db = FilterAndPaginate(db, &models.Site{}, c, "hostname")
	result := db.Find(&sites)
	if result.Error != nil {
		api.SendInternalServerError(c, errors.New("error fetching keys from db"))
		return
	}

	tokenClaims, err := NxodusClaims(c, api.db.WithContext(ctx))
	if err != nil {
		c.JSON(err.Status, err.Body)
		return
	}

	// only show the site token when using the reg token that created the site.
	userId := api.GetCurrentUserID(c)
	for i := range sites {
		hideSiteBearerToken(&sites[i], tokenClaims, userId)
	}
	c.JSON(http.StatusOK, sites)
}

func encryptSiteBearerToken(token string, publicKey string) string {
	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return ""
	}
	sealed, err := wgcrypto.SealV1(key[:], []byte(token))
	if err != nil {
		return ""
	}

	return sealed.String()
}

func hideSiteBearerToken(site *models.Site, claims *models.NexodusClaims, currentUserId uuid.UUID) {

	var hide bool
	if claims != nil {
		switch claims.Scope {
		case "reg-token":
			hide = claims.ID != site.RegKeyID.String()
		case "device-token":
			hide = claims.ID != site.ID.String()
		default:
			hide = currentUserId != site.OwnerID
		}
	} else {
		hide = currentUserId != site.OwnerID
	}

	if hide {
		site.BearerToken = ""
	} else {
		site.BearerToken = encryptSiteBearerToken(site.BearerToken, site.PublicKey)
	}
}

func (api *API) SiteIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
	userId := api.GetCurrentUserID(c)
	return db.Where("owner_id = ?", userId)
}

// GetSite gets a site by ID
// @Summary      Get Sites
// @Description  Gets a site by ID
// @Id  		 GetSite
// @Tags         Sites
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Site ID"
// @Success      200  {object}  models.Site
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/sites/{id} [get]
func (api *API) GetSite(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "GetSite", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()

	if !api.FlagCheck(c, "sites") {
		return
	}

	k, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var site models.Site

	db := api.db.WithContext(ctx)
	db = api.SiteIsOwnedByCurrentUser(c, db)
	result := db.First(&site, "id = ?", k)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		c.Status(http.StatusNotFound)
		return
	}

	tokenClaims, err2 := NxodusClaims(c, api.db.WithContext(ctx))
	if err2 != nil {
		c.JSON(err2.Status, err2.Body)
		return
	}

	// only show the site token when using the reg token that created the site.
	userId := api.GetCurrentUserID(c)
	hideSiteBearerToken(&site, tokenClaims, userId)

	c.JSON(http.StatusOK, site)
}

// UpdateSite updates a Site
// @Summary      Update Sites
// @Description  Updates a site by ID
// @Id  		 UpdateSite
// @Tags         Sites
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Site ID"
// @Param		 update body models.UpdateSite true "Site Update"
// @Success      200  {object}  models.Site
// @Failure		 401  {object}  models.BaseError
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/sites/{id} [patch]
func (api *API) UpdateSite(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "UpdateSite", trace.WithAttributes(
		attribute.String("id", c.Param("id")),
	))
	defer span.End()

	if !api.FlagCheck(c, "sites") {
		return
	}

	siteId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var request models.UpdateSite

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	var site models.Site
	var tokenClaims *models.NexodusClaims
	err = api.transaction(ctx, func(tx *gorm.DB) error {

		db := api.SiteIsOwnedByCurrentUser(c, tx)
		db = FilterAndPaginate(db, &models.Site{}, c, "hostname")

		result := db.First(&site, "id = ?", siteId)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("site"))
		}

		var err2 *ApiResponseError
		tokenClaims, err2 = NxodusClaims(c, tx)
		if err2 != nil {
			return err2
		}

		if tokenClaims != nil {
			switch tokenClaims.Scope {
			case "reg-token":
				if tokenClaims.ID != site.RegKeyID.String() {
					return NewApiResponseError(http.StatusForbidden, models.NewApiError(errors.New("reg key does not have access")))
				}
			case "device-token":
				if tokenClaims.ID != site.ID.String() {
					return NewApiResponseError(http.StatusForbidden, models.NewApiError(errors.New("reg key does not have access")))
				}
			}
		}

		var ServiceNetwork models.ServiceNetwork
		if result = tx.First(&ServiceNetwork, "id = ?", site.ServiceNetworkID); result.Error != nil {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("service_network"))
		}

		if request.Hostname != nil {
			site.Hostname = *request.Hostname
		}
		if request.Os != nil {
			site.Os = *request.Os
		}
		if request.LinkSecret != nil {
			site.LinkSecret = *request.LinkSecret
		}

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Save(&site); res.Error != nil {
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

	hideSiteBearerToken(&site, tokenClaims, api.GetCurrentUserID(c))

	api.signalBus.Notify(fmt.Sprintf("/sites/service-network=%s", site.ServiceNetworkID.String()))
	c.JSON(http.StatusOK, site)
}

// CreateSite handles adding a new site
// @Summary      Add Sites
// @Id  		 CreateSite
// @Tags         Sites
// @Description  Adds a new site
// @Accept       json
// @Produce      json
// @Param        Site  body   models.AddSite  true "Add Site"
// @Success      201  {object}  models.Site
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure      409  {object}  models.ConflictsError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/sites [post]
func (api *API) CreateSite(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "AddSite")
	defer span.End()

	if !api.FlagCheck(c, "sites") {
		return
	}

	var request models.AddSite
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	if request.PublicKey == "" {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("public_key"))
		return
	}
	if request.ServiceNetworkID == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.NewFieldNotPresentError("service_network_id"))
		return
	}

	var tokenClaims *models.NexodusClaims
	var site models.Site
	err := api.transaction(ctx, func(tx *gorm.DB) error {

		var ServiceNetwork models.ServiceNetwork
		if result := api.ServiceNetworkIsReadableByCurrentUser(c, tx).
			Preload("Organization").
			First(&ServiceNetwork, "id = ?", request.ServiceNetworkID); result.Error != nil {
			return NewApiResponseError(http.StatusNotFound, models.NewNotFoundError("service_network"))
		}

		res := tx.Where("public_key = ?", request.PublicKey).First(&site)
		if res.Error == nil {
			return NewApiResponseError(http.StatusConflict, models.NewConflictsError(site.ID.String()))
		}
		if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return res.Error
		}

		var err2 *ApiResponseError
		tokenClaims, err2 = NxodusClaims(c, tx)
		if err2 != nil {
			return err2
		}
		if tokenClaims != nil && tokenClaims.Scope != "reg-token" {
			tokenClaims = nil
		}

		siteId := uuid.Nil
		regKeyID := uuid.Nil
		var err error
		if tokenClaims != nil {
			regKeyID, err = uuid.Parse(tokenClaims.ID)
			if err != nil {
				return NewApiResponseError(http.StatusBadRequest, fmt.Errorf("invalid reg key id"))
			}

			// is the user token restricted to operating on a single site?
			if tokenClaims.AgentID != nil {
				err = tx.Where("id = ?", tokenClaims.AgentID).First(&site).Error
				if err == nil {
					// If we get here the site exists but has a different public key, so assume
					// the reg toke has been previously used.
					return NewApiResponseError(http.StatusBadRequest, models.NewApiError(errRegKeyExhausted))
				}

				siteId = *tokenClaims.AgentID
			}

			if tokenClaims.ServiceNetworkID == nil || *tokenClaims.ServiceNetworkID != request.ServiceNetworkID {
				return NewApiResponseError(http.StatusBadRequest, models.NewFieldValidationError("service_network_id", "does not match the reg key service_network_id"))
			}
		}
		if siteId == uuid.Nil {
			siteId = uuid.New()
		}

		// lets use a wg private key as the token, since it should be hard to guess.
		siteToken, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}

		site = models.Site{
			Base: models.Base{
				ID: siteId,
			},
			OwnerID:          api.GetCurrentUserID(c),
			ServiceNetworkID: ServiceNetwork.ID,
			OrganizationID:   ServiceNetwork.OrganizationID,
			PublicKey:        request.PublicKey,
			Platform:         request.Platform,
			Name:             request.Name,
			RegKeyID:         regKeyID,
			BearerToken:      "ST:" + siteToken.String(),
		}

		if res := tx.
			Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
			Create(&site); res.Error != nil {
			return res.Error
		}
		span.SetAttributes(
			attribute.String("id", site.ID.String()),
		)
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

	hideSiteBearerToken(&site, tokenClaims, api.GetCurrentUserID(c))

	api.signalBus.Notify(fmt.Sprintf("/sites/service-network=%s", site.ServiceNetworkID.String()))
	c.JSON(http.StatusCreated, site)
}

// DeleteSite handles deleting an existing site and associated ipam lease
// @Summary      Delete Site
// @Description  Deletes an existing site and associated IPAM lease
// @Id 			 DeleteSite
// @Tags         Sites
// @Accept       json
// @Produce      json
// @Param        id   path      string  true "Site ID"
// @Success      204  {object}  models.Site
// @Failure      400  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/sites/{id} [delete]
func (api *API) DeleteSite(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "DeleteSite")
	defer span.End()

	if !api.FlagCheck(c, "sites") {
		return
	}

	siteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}

	site := models.Site{}
	db := api.db.WithContext(ctx)
	if res := api.SiteIsOwnedByCurrentUser(c, db).
		First(&site, "id = ?", siteID); res.Error != nil {

		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("site"))
		} else {
			c.JSON(http.StatusBadRequest, models.NewApiError(res.Error))
		}
		return
	}

	var ServiceNetwork models.ServiceNetwork
	result := db.
		First(&ServiceNetwork, "id = ?", site.ServiceNetworkID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		api.SendInternalServerError(c, result.Error)
	}

	// Null out unique fields to that a new site can be created later with the same values
	if res := api.db.WithContext(ctx).
		Model(&site).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "revision"}}}).
		Where("id = ?", site.Base.ID).
		Updates(map[string]interface{}{
			"bearer_token": nil,
			"public_key":   nil,
			"deleted_at":   gorm.DeletedAt{Time: time.Now(), Valid: true},
		}); res.Error != nil {
		api.SendInternalServerError(c, res.Error)
		return
	}

	api.signalBus.Notify(fmt.Sprintf("/sites/service-network=%s", site.ServiceNetworkID.String()))

	c.JSON(http.StatusOK, site)
}

// ListSitesInServiceNetwork lists all sites in an ServiceNetwork
// @Summary      List Sites
// @Description  Lists all sites for this ServiceNetwork
// @Id           ListSitesInServiceNetwork
// @Tags         ServiceNetwork
// @Accept       json
// @Produce      json
// @Param		 gt_revision     query  uint64   false "greater than revision"
// @Param		 id              path   string true "Service Network ID"
// @Success      200  {object}  []models.Site
// @Failure      400  {object}  models.BaseError
// @Failure		 401  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /api/ServiceNetworks/{id}/sites [get]
func (api *API) ListSitesInServiceNetwork(c *gin.Context) {

	ctx, span := tracer.Start(c.Request.Context(), "ListSitesInServiceNetwork",
		trace.WithAttributes(
			attribute.String("service_network_id", c.Param("id")),
		))
	defer span.End()

	if !api.FlagCheck(c, "sites") {
		return
	}

	ServiceNetworkId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
		return
	}
	var ServiceNetwork models.ServiceNetwork
	db := api.db.WithContext(ctx)
	result := api.ServiceNetworkIsReadableByCurrentUser(c, db).
		First(&ServiceNetwork, "id = ?", ServiceNetworkId.String())

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.NewNotFoundError("service_network"))
		} else {
			api.SendInternalServerError(c, result.Error)
		}
		return
	}

	var query Query
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, models.NewApiError(err))
		return
	}

	tokenClaims, err2 := NxodusClaims(c, api.db.WithContext(ctx))
	if err2 != nil {
		c.JSON(err2.Status, err2.Body)
		return
	}

	api.sendList(c, ctx, func(db *gorm.DB) (fetchmgr.ResourceList, error) {
		db = db.Where("service_network_id = ?", ServiceNetworkId.String())
		db = FilterAndPaginateWithQuery(db, &models.Site{}, c, query, "hostname")

		var items siteList
		result := db.Find(&items)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}

		userId := api.GetCurrentUserID(c)
		for i := range items {
			hideSiteBearerToken(items[i], tokenClaims, userId)
		}
		return items, nil
	})

}
