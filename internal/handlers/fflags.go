package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
)

// ListFeatureFlags lists all feature flags
// @Summary      List Feature Flags
// @Description  Lists all feature flags
// @Tags         FFlag
// @Accepts		 json
// @Produce      json
// @Success      200  {object} map[string]bool
// @Failure		 429  {object}  models.BaseError
// @Router       /fflags [get]
func (api *API) ListFeatureFlags(c *gin.Context) {
	c.JSON(http.StatusOK, api.fflags.ListFlags())
}

// GetFeatureFlag gets a feature flag by name
// @Summary      Get Feature Flag
// @Description  Gets a Feature Flag by name
// @Tags         FFlag
// @Accepts		 json
// @Produce      json
// @Param		 name path      string true  "feature flag name"
// @Success      200  {object} map[string]bool
// @Failure      400  {object}  models.BaseError
// @Failure      404  {object}  models.BaseError
// @Failure		 429  {object}  models.BaseError
// @Router       /fflags/{name} [get]
func (api *API) GetFeatureFlag(c *gin.Context) {
	flagName := c.Param("name")
	if flagName == "" {
		c.JSON(http.StatusBadRequest, models.NewBadPathParameterError("name"))
		return
	}

	enabled, err := api.fflags.GetFlag(flagName)
	if err != nil {
		c.JSON(http.StatusNotFound, models.NewNotFoundError("flag"))
		return
	}

	c.JSON(http.StatusOK, map[string]bool{flagName: enabled})
}
