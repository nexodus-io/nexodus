package handlers

import (
	"bytes"
	"encoding/json"
	//"fmt"
	"io"
	"net/http"

	//"github.com/nexodus-io/nexodus/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
)

func (suite *HandlerTestSuite) TestCreateStatus() {
	require := suite.Require()

	newStatus := models.Status{
		UserId:      suite.testUserID,
		WgIP:        "1.23.3",
		IsReachable: true,
		Hostname:    "Tester",
		Latency:     "1",
		Method:      "Internet",
	}

	resBody, err := json.Marshal(newStatus)
	require.NoError(err)

	_, res, err := suite.ServeRequest(
		http.MethodPost,
		"/status", "/status",
		func(c *gin.Context) {
			suite.api.CreateStatus(c)
		},
		bytes.NewBuffer(resBody),
	)
	require.NoError(err)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)

	require.Equal(http.StatusCreated, res.Code, "HTTP error: %s", string(body))

	var actual models.Status
	err = json.Unmarshal(body, &actual)
	require.NoError(err)

	// Add more assertions as needed
	require.Equal(newStatus.WgIP, actual.WgIP)

}
