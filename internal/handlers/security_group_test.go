package handlers

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
)

func (suite *HandlerTestSuite) TestListSecurityGroups() {
	assert := suite.Assert()
	require := suite.Require()

	organization := models.AddOrganization{
		Name:     "test-organization",
		IpCidr:   "10.1.1.0/24",
		IpCidrV6: "fc00::/20",
	}

	var orgId uuid.UUID
	{
		reqBody, err := json.Marshal(organization)
		assert.NoError(err)
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			func(c *gin.Context) {
				c.Set("nexodus.testCreateOrganization", "true")
				suite.api.CreateOrganization(c)
			},
			bytes.NewBuffer(reqBody),
		)
		require.NoError(err)
		body, err := io.ReadAll(res.Body)
		require.NoError(err)
		require.Equal(http.StatusCreated, res.Code, string(body))

		var org models.OrganizationJSON
		err = json.Unmarshal(body, &org)
		require.NoError(err)
		orgId = org.ID
	}

	securityGroups := []models.AddSecurityGroup{
		{
			GroupName:        "security-group-a",
			GroupDescription: "Description for group A",
			OrganizationId:   orgId,
			InboundRules:     `[{"ip_protocol": "tcp", "from_port": 80, "to_port": 80, "ip_ranges": ["10.0.0.0/8"], "prefix": ["2001:db8::/32"]}]`,
			OutboundRules:    `[{"ip_protocol": "tcp", "from_port": 80, "to_port": 80, "ip_ranges": ["0.0.0.0/0"], "prefix": ["::/0"]}]`,
		},
		{
			GroupName:        "security-group-b",
			GroupDescription: "Description for group B",
			OrganizationId:   orgId,
			InboundRules:     `[{"ip_protocol": "tcp", "from_port": 443, "to_port": 443, "ip_ranges": ["10.0.0.0/8"], "prefix": ["2001:db8::/32"]}]`,
			OutboundRules:    `[{"ip_protocol": "tcp", "from_port": 443, "to_port": 443, "ip_ranges": ["0.0.0.0/0"], "prefix": ["::/0"]}]`,
		},
	}

	for _, securityGroup := range securityGroups {
		reqBody, err := json.Marshal(securityGroup)
		assert.NoError(err)
		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/", "/",
			func(c *gin.Context) {
				c.Set("nexodus.testCreateSecurityGroup", "true")
				suite.api.CreateSecurityGroup(c)
			},
			bytes.NewBuffer(reqBody),
		)
		require.NoError(err)
		body, err := io.ReadAll(res.Body)
		require.NoError(err)
		require.Equal(http.StatusCreated, res.Code, string(body))

		var sg models.SecurityGroup
		err = json.Unmarshal(body, &sg)
		require.NoError(err)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", "/",
			suite.api.ListSecurityGroups, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.SecurityGroup
		err = json.Unmarshal(body, &actual)
		assert.NoError(err)
		assert.Len(actual, 16)
	}

	{
		_, res, err := suite.ServeRequest(
			http.MethodGet,
			"/", `/?range=[3,4]`,
			suite.api.ListSecurityGroups, nil,
		)
		assert.NoError(err)

		body, err := io.ReadAll(res.Body)
		assert.NoError(err)
		assert.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var actual []models.SecurityGroup
		err = json.Unmarshal(body, &actual)

		assert.NoError(err)
		assert.Len(actual, 16)
	}
}
