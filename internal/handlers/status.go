package handlers

import (
	"errors"
	//"fmt"
	//"github.com/nexodus-io/nexodus/internal/handlers/fetchmgr"
	"net/http"
	//"time"
	//"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	//"github.com/google/uuid"
	//"github.com/nexodus-io/nexodus/internal/database"
	"github.com/nexodus-io/nexodus/internal/models"
	//"github.com/nexodus-io/nexodus/internal/util"
	//"github.com/nexodus-io/nexodus/internal/wgcrypto"
	//"go.opentelemetry.io/otel/attribute"
	//"go.opentelemetry.io/otel/trace"
	//"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
	//"gorm.io/gorm/clause"
)

func (api *API) CreateStatus(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "AddStatus")
	defer span.End()

	if c.Request.Method != http.MethodPost {
        // Respond with a 405 Method Not Allowed error
        c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{"error": "Method Not Allowed"})
        return
    }

	var request models.Status
	// Call BindJSON to bind the received JSON
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, models.NewBadPayloadError(err))
		return
	}

	userId := api.GetCurrentUserID(c);
	var status models.Status

	err := api.transaction(ctx, func(tx *gorm.DB) error {
		
		status= models.Status{
			UserId:			userId,
			WgIP:        	request.WgIP,
			IsReachable: 	request.IsReachable,
			Hostname:		request.Hostname,
			Latency:		request.Latency,
			Method:			request.Method,
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

	c.JSON(http.StatusCreated, status)

}

func (api *API) GetStatus(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "ListStatus", trace.WithAttributes(
        attribute.String("id", c.Param("id")),
    ))
    defer span.End()

    if !api.FlagCheck(c, "status") {
        return
    }

    k, err := uuid.Parse(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("id"))
        return
    }

    var status models.Status

    db := api.db.WithContext(ctx)
    db = api.StatusIsOwnedByCurrentUser(c, db)
    result := db.First(&status, "id = ?", k)
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        c.Status(http.StatusNotFound)
        return
    }

    tokenClaims, err2 := NxodusClaims(c, api.db.WithContext(ctx))
    if err2 != nil {
        c.JSON(err2.Status, err2.Body)
        return
    }

    // only show the status token when using the reg token that created the status|
    userId := api.GetCurrentUserID(c)
    hideStatusBearerToken(&status, tokenClaims, userId)

    c.JSON(http.StatusOK, status)
}

func (api *API) StatusIsOwnedByCurrentUser(c *gin.Context, db *gorm.DB) *gorm.DB {
    userId := api.GetCurrentUserID(c)
    return db.Where("owner_id = ?", userId)
}

