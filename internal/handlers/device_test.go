package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
)

func (suite *HandlerTestSuite) TestCreateGetDevice() {
	assert := suite.Assert()
	newDevice := models.AddDevice{
		PublicKey: "atestpubkey",
	}

	resBody, err := json.Marshal(newDevice)
	assert.NoError(err)

	_, res, err := suite.ServeRequest(
		http.MethodPost,
		"/", "/",
		suite.api.CreateDevice, bytes.NewBuffer(resBody),
	)
	assert.NoError(err)

	body, err := io.ReadAll(res.Body)
	assert.NoError(err)

	assert.Equal(http.StatusCreated, res.Code, "HTTP error: %s", string(body))

	var actual models.Device
	err = json.Unmarshal(body, &actual)
	assert.NoError(err)

	assert.Equal(newDevice.PublicKey, actual.PublicKey)
	assert.Equal(uuid.MustParse(TestUserID), actual.UserID)

	_, res, err = suite.ServeRequest(
		http.MethodGet,
		"/:id", fmt.Sprintf("/%s", actual.ID),
		suite.api.GetDevice, nil,
	)

	assert.NoError(err)
	body, err = io.ReadAll(res.Body)
	assert.NoError(err)

	assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

	var device models.Device
	err = json.Unmarshal(body, &device)
	assert.NoError(err)

	assert.Equal(actual, device)
}
