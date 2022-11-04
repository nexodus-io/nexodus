package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/redhat-et/apex/internal/models"
)

func (suite *HandlerTestSuite) TestListZones() {
	assert := suite.Assert()
	zones := []models.AddZone{
		{
			Name:   "zone-a",
			IpCidr: "10.1.1.0/24",
		},
		{
			Name:   "zone-b",
			IpCidr: "10.1.2.0/24",
		},
		{
			Name:   "zone-c",
			IpCidr: "10.1.3.0/24",
		},
	}

	for _, zone := range zones {
		resBody, err := json.Marshal(zone)
		assert.NoError(err)
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			suite.api.CreateZone, bytes.NewBuffer(resBody),
		)
		assert.NoError(err)
		assert.Equal(http.StatusCreated, res.Code)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", "/",
			suite.api.ListZones, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.Zone
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)
		// 3 zones + default
		assert.Len(actual, 4)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?sort=["name","DESC"]`,
			suite.api.ListZones, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.Zone
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)

		assert.Len(actual, 4)
		assert.Equal("zone-c", actual[0].Name)
		assert.Equal("zone-b", actual[1].Name)
		assert.Equal("zone-a", actual[2].Name)
		assert.Equal("default", actual[3].Name)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?filter={"name":["default"}`,
			suite.api.ListZones, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.Zone
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)

		assert.Len(actual, 1)
		assert.Equal("default", actual[0].Name)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?range=[3,4]`,
			suite.api.ListZones, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.Zone
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)

		assert.Len(actual, 1)
		assert.Equal("4", res.Header().Get(TotalCountHeader))
		assert.Equal("zone-c", actual[0].Name)
	}
}
