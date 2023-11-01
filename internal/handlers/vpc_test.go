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
			Description: "vpc-a",
			PrivateCidr: true,
			IpCidr:      "10.1.1.0/24",
			IpCidrV6:    "fc00::/20",
		},
		{
			Description: "vpc-b",
			PrivateCidr: true,
			IpCidr:      "10.1.2.0/24",
			IpCidrV6:    "fc00:1000::/20",
		},
		{
			Description: "vpc-c",
			PrivateCidr: true,
			IpCidr:      "10.1.3.0/24",
			IpCidrV6:    "fc00:2000::/20",
		},
	}
	vpcDenied := models.AddVPC{
		Description: "vpc-denied-multi-vpc-off",
		IpCidr:      "10.1.3.0/24",
		IpCidrV6:    "fc00:3000::/20",
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
				c.Set("nexodus.testCreateVPC", "false")
				suite.api.CreateVPC(c)
			},

			bytes.NewBuffer(resBody),
		)
		assert.NoError(err)
		assert.Equal(http.StatusMethodNotAllowed, res.Code)
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
			"/", `/?sort=["name","DESC"]`,
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
			"testuser": false,
			"vpc-a":    false,
			"vpc-b":    false,
			"vpc-c":    false,
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
			"/", `/?filter={"name":"default"}`,
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
			http.MethodGet,
			"/", `/?range=[3,4]`,
			suite.api.ListVPCs, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.VPC
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)
		// The orgs are sorted by name..
		assert.Len(actual, 1)
		assert.Equal("4", res.Header().Get(TotalCountHeader))
		assert.Equal("testuser", actual[0].Description)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			suite.api.CreateVPC,
			bytes.NewBuffer(suite.jsonMarshal(models.AddVPC{
				Description: "bad-ipv4-cidr",
				IpCidr:      "10.1.3.0/24",
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
				IpCidrV6:    "fc00::/20",
			})),
		)
		assert.NoError(err)
		assert.Equal(http.StatusBadRequest, res.Code)
		assert.Equal(`{"error":"must be '200::/64' or not set when private_cidr is not enabled","field":"cidr_v6"}`, res.Body.String())

	}
}
