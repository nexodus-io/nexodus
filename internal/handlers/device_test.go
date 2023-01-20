package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/redhat-et/apex/internal/models"
)

func (suite *HandlerTestSuite) TestCreateGetDevice() {
	require := suite.Require()
	assert := suite.Assert()
	newDevice := models.AddDevice{
		OrganizationID: suite.testOrganizationID,
		PublicKey:      "atestpubkey",
	}

	resBody, err := json.Marshal(newDevice)
	require.NoError(err)

	_, res, err := suite.ServeRequest(
		http.MethodPost,
		"/", "/",
		suite.api.CreateDevice, bytes.NewBuffer(resBody),
	)
	require.NoError(err)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)

	require.Equal(http.StatusCreated, res.Code, "HTTP error: %s", string(body))

	var actual models.Device
	err = json.Unmarshal(body, &actual)
	require.NoError(err)

	require.Equal(newDevice.PublicKey, actual.PublicKey)
	require.Equal(TestUserID, actual.UserID)

	_, res, err = suite.ServeRequest(
		http.MethodGet, "/:id", fmt.Sprintf("/%s", actual.ID),
		suite.api.GetDevice, nil,
	)

	require.NoError(err)
	body, err = io.ReadAll(res.Body)
	require.NoError(err)

	require.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

	var device models.Device
	err = json.Unmarshal(body, &device)
	require.NoError(err)

	assert.Equal(actual, device)
}
