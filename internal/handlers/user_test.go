package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nexodus-io/nexodus/internal/models"
)

func (suite *HandlerTestSuite) TestGetUser() {
	require := suite.Require()
	assert := suite.Assert()
	_, res, err := suite.ServeRequest(
		http.MethodPost,
		"/:id", fmt.Sprintf("/%s", suite.testUserID),
		suite.api.GetUser, nil,
	)
	require.NoError(err)
	body, err := io.ReadAll(res.Body)
	require.NoError(err)
	require.Equal(http.StatusOK, res.Code, string(body))

	var actual models.User
	err = json.Unmarshal(body, &actual)
	require.NoError(err, string(body))

	assert.NotEmpty(actual.ID)
}
