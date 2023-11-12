package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
)

func (suite *HandlerTestSuite) TestListVPCs() {
	assert := suite.Assert()
	require := suite.Require()
	vpcs := []models.AddVPC{
		{
			Description:    "vpc-a",
			PrivateCidr:    true,
			Ipv4Cidr:       "10.1.1.0/24",
			Ipv6Cidr:       "fc00::/20",
			OrganizationID: suite.testUserID,
		},
		{
			Description:    "vpc-b",
			PrivateCidr:    true,
			Ipv4Cidr:       "10.1.2.0/24",
			Ipv6Cidr:       "fc00:1000::/20",
			OrganizationID: suite.testUserID,
		},
		{
			Description:    "vpc-c",
			PrivateCidr:    true,
			Ipv4Cidr:       "10.1.3.0/24",
			Ipv6Cidr:       "fc00:2000::/20",
			OrganizationID: suite.testUserID,
		},
	}
	vpcDenied := models.AddVPC{
		Description:    "vpc-denied-multi-vpc-off",
		Ipv4Cidr:       "10.1.3.0/24",
		Ipv6Cidr:       "fc00:3000::/20",
		OrganizationID: suite.testUserID,
	}

	for _, vpc := range vpcs {
		reqBody, err := json.Marshal(vpc)
		assert.NoError(err)
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			suite.api.CreateVPC,
			bytes.NewBuffer(reqBody),
		)
		require.NoError(err)
		body, err := io.ReadAll(res.Body)
		require.NoError(err)
		require.Equal(http.StatusCreated, res.Code, string(body))

		var o models.VPC
		err = json.Unmarshal(body, &o)
		require.NoError(err)
	}

	{
		resBody, err := json.Marshal(vpcDenied)
		assert.NoError(err)
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			func(c *gin.Context) {
				c.Set("nexodus.fflag.multi-organization", false)
				suite.api.CreateVPC(c)
			},

			bytes.NewBuffer(resBody),
		)
		assert.NoError(err)
		assert.Equal(http.StatusBadRequest, res.Code)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", "/",
			suite.api.ListVPCs, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.VPC
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)
		assert.Len(actual, 4)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?sort=["description","DESC"]`,
			suite.api.ListVPCs, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.VPC
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)

		assert.Len(actual, 4)
		seen := map[string]bool{
			"default vpc": false,
			"vpc-a":       false,
			"vpc-b":       false,
			"vpc-c":       false,
		}
		for _, org := range actual {
			if _, ok := seen[org.Description]; ok {
				seen[org.Description] = true
			}
		}
		for k, v := range seen {
			assert.Equal(v, true, "vpc %s was not seen", k)
		}
	}
	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?filter={"description":"default"}`,
			suite.api.ListVPCs, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.VPC
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)

		assert.Len(actual, 0)
	}
	{
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			suite.api.CreateVPC,
			bytes.NewBuffer(suite.jsonMarshal(models.AddVPC{
				Description: "bad-ipv4-cidr",
				Ipv4Cidr:    "10.1.3.0/24",
			})),
		)
		assert.NoError(err)
		assert.Equal(http.StatusBadRequest, res.Code)
		assert.Equal(`{"error":"must be '100.64.0.0/10' or not set when private_cidr is not enabled","field":"cidr"}`, res.Body.String())

	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			suite.api.CreateVPC,
			bytes.NewBuffer(suite.jsonMarshal(models.AddVPC{
				Description: "bad-ipv4-cidr",
				Ipv6Cidr:    "fc00::/20",
			})),
		)
		assert.NoError(err)
		assert.Equal(http.StatusBadRequest, res.Code)
		assert.Equal(`{"error":"must be '200::/64' or not set when private_cidr is not enabled","field":"cidr_v6"}`, res.Body.String())

	}
}
