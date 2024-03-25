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

func (suite *HandlerTestSuite) TestListStatues() {
	require := suite.Require()

	// Optional: Create a new status as part of setup for this test
	newStatus := models.Status{
		UserId:      suite.testUserID,
		WgIP:        "1.23.4",
		IsReachable: true,
		Hostname:    "ListTester",
		Latency:     "1",
		Method:      "Internet",
	}

	resBody, err := json.Marshal(newStatus)
	require.NoError(err)

	_, _, err = suite.ServeRequest(
		http.MethodPost,
		"/status", "/status",
		func(c *gin.Context) {
			suite.api.CreateStatus(c)
		},
		bytes.NewBuffer(resBody),
	)
	require.NoError(err)

	// Make a GET request to ListStatues
	_, res, err := suite.ServeRequest(
		http.MethodGet,
		"/status", "/status",
		func(c *gin.Context) {
			suite.api.ListStatues(c)
		},
		nil, // No body needed for GET request
	)
	require.NoError(err)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)

	require.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

	var actual []models.Status
	err = json.Unmarshal(body, &actual)
	require.NoError(err)
}
