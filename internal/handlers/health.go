package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Ready checks if the service is ready to accept requests
// @Summary      Checks if the service is ready to accept requests
// @Description  Checks if the service is ready to accept requests
// @Id           Ready
// @Tags         Private
// @Accept       json
// @Produce      json
// @Success      200
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /private/ready [post]
func (api *API) Ready(c *gin.Context) {
	api.Live(c)
}

// Live checks if the service is live
// @Summary      Checks if the service is live
// @Description  Checks if the service is live
// @Id           Live
// @Tags         Private
// @Accept       json
// @Produce      json
// @Success      200
// @Failure      500  {object}  models.InternalServerError "Internal Server Error"
// @Router       /private/live [post]
func (api *API) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "UP",
	})
}
