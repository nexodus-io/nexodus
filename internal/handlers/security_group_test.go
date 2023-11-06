package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nexodus-io/nexodus/internal/models"
)

func (suite *HandlerTestSuite) TestCreateGetSecurityGroups() {
	require := suite.Require()
	assert := suite.Assert()

	groups := []models.AddSecurityGroup{
		{
			Description:    "This is test group 1",
			OrganizationId: suite.testUserID,
			InboundRules: []models.SecurityRule{
				{IpProtocol: "tcp", FromPort: 22, ToPort: 22, IpRanges: []string{"192.168.100.0/24", "2001:db8::/32"}},
				{IpProtocol: "udp", FromPort: 5000, ToPort: 5001, IpRanges: []string{"10.0.0.0/8"}},
			},
			OutboundRules: []models.SecurityRule{
				{IpProtocol: "tcp", FromPort: 22, ToPort: 22, IpRanges: []string{"2001:db8::/32", "::/0"}},
				{IpProtocol: "udp", FromPort: 5000, ToPort: 5001, IpRanges: []string{"10.0.0.0/8", "100.101.0.0/24"}},
			},
		},
		{
			Description:    "This is test group 2",
			OrganizationId: suite.testUserID,
			InboundRules: []models.SecurityRule{
				{IpProtocol: "udp", FromPort: 99, ToPort: 100, IpRanges: []string{"2001:db8:a0b:12f0::/64", "2001:db8::/32"}},
				{IpProtocol: "icmp", FromPort: 0, ToPort: 0, IpRanges: []string{"192.168.1.0/24", "10.0.0.0/8"}},
			},
			OutboundRules: []models.SecurityRule{
				{IpProtocol: "udp", FromPort: 53, ToPort: 53, IpRanges: []string{"10.0.0.0/8", "192.168.1.0/24"}},
				{IpProtocol: "icmpv6", FromPort: 0, ToPort: 0, IpRanges: []string{"2001:db8::/32", "2001:db8:a0b:12f0::/64", "200::/64"}},
			},
		},
		{
			Description:    "This is test group 3",
			OrganizationId: suite.testUserID,
			InboundRules: []models.SecurityRule{
				{IpProtocol: "icmp", FromPort: 0, ToPort: 0, IpRanges: []string{"192.168.1.0/24", "2001:db8:a0b:12f0::/64"}},
				{IpProtocol: "tcp", FromPort: 443, ToPort: 443, IpRanges: []string{"0.0.0.0/0", "::/0"}},
			},
			OutboundRules: []models.SecurityRule{
				{IpProtocol: "icmp", FromPort: 0, ToPort: 0, IpRanges: []string{"2002:db8:a0b:12f0::/64", "2003:db8:a0b:12f0::1"}},
				{IpProtocol: "tcp", FromPort: 30000, ToPort: 60000, IpRanges: []string{"192.168.1.201", "100.100.0.50"}},
			},
		},
	}

	for _, newGroup := range groups {
		resBody, err := json.Marshal(newGroup)
		require.NoError(err)

		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/organizations/:organization/security-groups", fmt.Sprintf("/organizations/%s/security-groups", suite.testUserID.String()),
			func(c *gin.Context) {
				c.Set("nexodus.secGroupsEnabled", "true")
				suite.api.CreateSecurityGroup(c)
			},
			bytes.NewBuffer(resBody),
		)
		require.NoError(err)

		body, err := io.ReadAll(res.Body)
		require.NoError(err)

		require.Equal(http.StatusCreated, res.Code, "HTTP error: %s", string(body))

		var actual models.SecurityGroup
		err = json.Unmarshal(body, &actual)
		require.NoError(err)

		require.Equal(newGroup.Description, actual.Description)
		require.Equal(newGroup.OrganizationId, actual.OrganizationId)

		_, res, err = suite.ServeRequest(
			http.MethodGet, "/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actual.ID),
			suite.api.GetSecurityGroup, nil,
		)

		require.NoError(err)
		body, err = io.ReadAll(res.Body)
		require.NoError(err)

		require.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

		var group models.SecurityGroup
		err = json.Unmarshal(body, &group)
		require.NoError(err)

		assert.Equal(actual, group)
	}
}

func (suite *HandlerTestSuite) TestDeleteSecurityGroup() {
	require := suite.Require()

	// create a security group that we will delete later
	newGroup := models.AddSecurityGroup{
		Description:    "This is a test group to delete",
		OrganizationId: suite.testUserID,
		InboundRules:   []models.SecurityRule{{IpProtocol: "tcp", FromPort: 0, ToPort: 0, IpRanges: []string{}}},
		OutboundRules:  []models.SecurityRule{{IpProtocol: "udp", FromPort: 0, ToPort: 0, IpRanges: []string{}}},
	}

	resBody, err := json.Marshal(newGroup)
	require.NoError(err)

	_, res, err := suite.ServeRequest(
		http.MethodPost,
		"/organizations/:organization/security-groups", fmt.Sprintf("/organizations/%s/security-groups", suite.testUserID.String()),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.CreateSecurityGroup(c)
		},
		bytes.NewBuffer(resBody),
	)
	require.NoError(err)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)

	require.Equal(http.StatusCreated, res.Code, "HTTP error: %s", string(body))

	var actual models.SecurityGroup
	err = json.Unmarshal(body, &actual)
	require.NoError(err)

	// Now delete the security group we just created
	_, res, err = suite.ServeRequest(
		http.MethodDelete,
		"/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actual.ID),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.DeleteSecurityGroup(c)
		},
		nil,
	)

	require.NoError(err)
	require.Equal(http.StatusOK, res.Code, "HTTP error: %s", string(body))

	// Verify that the security group has been deleted by trying to get it
	_, res, err = suite.ServeRequest(
		http.MethodGet, "/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actual.ID),
		suite.api.GetSecurityGroup, nil,
	)

	require.NoError(err)
	require.Equal(http.StatusNotFound, res.Code, "HTTP error: %s", string(body))
}

func (suite *HandlerTestSuite) TestListSecurityGroups() {
	require := suite.Require()
	assert := suite.Assert()

	// Create a couple of security groups for testing
	groups := []models.AddSecurityGroup{
		{
			Description:    "This is test group 1",
			OrganizationId: suite.testUserID,
			InboundRules:   []models.SecurityRule{{IpProtocol: "tcp", FromPort: 80, ToPort: 80, IpRanges: []string{"100.100.0.0/16"}}},
			OutboundRules:  []models.SecurityRule{{IpProtocol: "tcp", FromPort: 80, ToPort: 80, IpRanges: []string{}}},
		},
		{
			Description:    "This is test group 2",
			OrganizationId: suite.testUserID,
			InboundRules:   []models.SecurityRule{{IpProtocol: "tcp", FromPort: 443, ToPort: 443, IpRanges: []string{"200::1-200::5"}}},
			OutboundRules:  []models.SecurityRule{{IpProtocol: "tcp", FromPort: 443, ToPort: 8080, IpRanges: []string{"172.20.0.1-172.20.0.100"}}},
		},
	}

	for _, newGroup := range groups {
		resBody, err := json.Marshal(newGroup)
		require.NoError(err)

		_, res, err := suite.ServeRequest(
			http.MethodPost,
			"/organizations/:organization/security-groups", fmt.Sprintf("/organizations/%s/security-groups", suite.testUserID.String()),
			func(c *gin.Context) {
				c.Set("nexodus.secGroupsEnabled", "true")
				suite.api.CreateSecurityGroup(c)
			},
			bytes.NewBuffer(resBody),
		)
		require.NoError(err)
		require.Equal(http.StatusCreated, res.Code)
	}

	// List all the security groups
	_, res, err := suite.ServeRequest(
		http.MethodGet, "/organizations/:organization/security-groups", fmt.Sprintf("/organizations/%s/security-groups", suite.testUserID.String()),
		suite.api.ListSecurityGroups, nil,
	)

	require.NoError(err)
	require.Equal(http.StatusOK, res.Code)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)

	// Unmarshal the response body
	var actualGroups []models.SecurityGroup
	err = json.Unmarshal(body, &actualGroups)
	require.NoError(err)

	// Check that we got back the right number of security groups +1 for the default org security group
	assert.Len(actualGroups, len(groups)+1)
}

func (suite *HandlerTestSuite) TestUpdateSecurityGroup() {
	require := suite.Require()
	assert := suite.Assert()

	// Create a new security group
	newGroup := models.AddSecurityGroup{
		Description:    "This is a test group",
		OrganizationId: suite.testUserID,
		InboundRules:   []models.SecurityRule{{IpProtocol: "tcp", FromPort: 22, ToPort: 80, IpRanges: []string{"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db8:ffff:ffff:ffff:ffff:ffff:ffff"}}},
		OutboundRules:  []models.SecurityRule{{IpProtocol: "tcp", FromPort: 0, ToPort: 0, IpRanges: []string{"192.168.50.1-192.168.50.100"}}},
	}

	resBody, err := json.Marshal(newGroup)
	require.NoError(err)

	_, res, err := suite.ServeRequest(
		http.MethodPost,
		"/organizations/:organization/security-groups", fmt.Sprintf("/organizations/%s/security-groups", suite.testUserID.String()),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.CreateSecurityGroup(c)
		},
		bytes.NewBuffer(resBody),
	)
	require.NoError(err)
	require.Equal(http.StatusCreated, res.Code)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)

	var actualGroup models.SecurityGroup
	err = json.Unmarshal(body, &actualGroup)
	require.NoError(err)

	// Update the security group
	updateGroup := models.UpdateSecurityGroup{
		Description:   util.PtrString("This is an updated test group"),
		InboundRules:  []models.SecurityRule{{IpProtocol: "tcp", FromPort: 22, ToPort: 22, IpRanges: []string{"10.130.0.1-10.130.0.5", "192.168.64.10-192.168.64.50", "100.100.0.128/25"}}},
		OutboundRules: []models.SecurityRule{{IpProtocol: "", FromPort: 0, ToPort: 0, IpRanges: []string{"200::/64", "201::1-201::8"}}},
	}

	updateBody, err := json.Marshal(updateGroup)
	require.NoError(err)

	_, res, err = suite.ServeRequest(
		http.MethodPatch,
		"/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actualGroup.ID),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.UpdateSecurityGroup(c)
		},
		bytes.NewBuffer(updateBody),
	)
	require.NoError(err)
	require.Equal(http.StatusOK, res.Code)

	body, err = io.ReadAll(res.Body)
	require.NoError(err)

	var updatedGroup models.SecurityGroup
	err = json.Unmarshal(body, &updatedGroup)
	require.NoError(err)

	// Check the updated fields
	assert.Equal(*updateGroup.Description, updatedGroup.Description)
	assert.Equal(updateGroup.InboundRules, updatedGroup.InboundRules)
	assert.Equal(updateGroup.OutboundRules, updatedGroup.OutboundRules)
}

// TestInvalidUpdateSecurityGroup negative tests to ensure the proper code 422 is being returned
func (suite *HandlerTestSuite) TestInvalidUpdateSecurityGroup() {
	require := suite.Require()

	// Create a new security group
	newGroup := models.AddSecurityGroup{
		Description:    "This is a test group",
		OrganizationId: suite.testUserID,
		InboundRules:   []models.SecurityRule{{IpProtocol: "tcp", FromPort: 22, ToPort: 80, IpRanges: []string{"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db8:ffff:ffff:ffff:ffff:ffff:ffff"}}},
		OutboundRules:  []models.SecurityRule{{IpProtocol: "tcp", FromPort: 0, ToPort: 0, IpRanges: []string{"192.168.50.1-192.168.50.100"}}},
	}

	resBody, err := json.Marshal(newGroup)
	require.NoError(err)

	_, res, err := suite.ServeRequest(
		http.MethodPost,
		"/organizations/:organization/security-groups", fmt.Sprintf("/organizations/%s/security-groups", suite.testUserID.String()),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.CreateSecurityGroup(c)
		},
		bytes.NewBuffer(resBody),
	)
	require.NoError(err)
	require.Equal(http.StatusCreated, res.Code)

	body, err := io.ReadAll(res.Body)
	require.NoError(err)

	var actualGroup models.SecurityGroup
	err = json.Unmarshal(body, &actualGroup)
	require.NoError(err)

	// Update the security group with an invalid rule (FromPort > ToPort)
	updateGroup := models.UpdateSecurityGroup{
		Description: util.PtrString("This is an updated test group"),
		InboundRules: []models.SecurityRule{
			{IpProtocol: "tcp", FromPort: 8081, ToPort: 8080, IpRanges: []string{"200::/64"}},
		},
		OutboundRules: []models.SecurityRule{{IpProtocol: "tcp", FromPort: 8080, ToPort: 9001, IpRanges: []string{"200::/64"}}},
	}

	updateBody, err := json.Marshal(updateGroup)
	require.NoError(err)

	_, res, err = suite.ServeRequest(
		http.MethodPatch,
		"/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actualGroup.ID),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.UpdateSecurityGroup(c)
		},
		bytes.NewBuffer(updateBody),
	)
	require.NoError(err)

	// Should be http.StatusStatusUnprocessableEntity.
	require.Equal(http.StatusUnprocessableEntity, res.Code)

	// Update the security group with an invalid port range with a from_port of 0
	updateGroup = models.UpdateSecurityGroup{
		Description: util.PtrString("This is an updated test group"),
		InboundRules: []models.SecurityRule{
			{IpProtocol: "tcp", FromPort: 0, ToPort: 8080, IpRanges: []string{"10.130.0.1-10.130.0.5,10.20.0.0/28"}},
		},
		OutboundRules: []models.SecurityRule{{IpProtocol: "tcp", FromPort: 8080, ToPort: 9001, IpRanges: []string{"200::/64"}}},
	}

	updateBody, err = json.Marshal(updateGroup)
	require.NoError(err)

	_, res, err = suite.ServeRequest(
		http.MethodPatch,
		"/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actualGroup.ID),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.UpdateSecurityGroup(c)
		},
		bytes.NewBuffer(updateBody),
	)
	require.NoError(err)

	// Should be http.StatusStatusUnprocessableEntity.
	require.Equal(http.StatusUnprocessableEntity, res.Code)

	// Update the security group with an invalid ip range
	updateGroup = models.UpdateSecurityGroup{
		Description: util.PtrString("This is an updated test group"),
		InboundRules: []models.SecurityRule{
			{IpProtocol: "tcp", FromPort: 8080, ToPort: 8080, IpRanges: []string{"200::/64¯\\_(ツ)_/¯"}},
		},
		OutboundRules: []models.SecurityRule{{IpProtocol: "tcp", FromPort: 8080, ToPort: 9001, IpRanges: []string{"200::/64"}}},
	}

	updateBody, err = json.Marshal(updateGroup)
	require.NoError(err)

	_, res, err = suite.ServeRequest(
		http.MethodPatch,
		"/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actualGroup.ID),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.UpdateSecurityGroup(c)
		},
		bytes.NewBuffer(updateBody),
	)
	require.NoError(err)

	// Should be http.StatusStatusUnprocessableEntity.
	require.Equal(http.StatusUnprocessableEntity, res.Code)

	// Update the security group with an invalid ip range
	updateGroup = models.UpdateSecurityGroup{
		Description: util.PtrString("This is an updated test group"),
		InboundRules: []models.SecurityRule{
			{IpProtocol: "tcp", FromPort: 8080, ToPort: 8080, IpRanges: []string{""}},
		},
		OutboundRules: []models.SecurityRule{{IpProtocol: "ipv3000", FromPort: 8080, ToPort: 9001, IpRanges: []string{"200::/64"}}},
	}

	updateBody, err = json.Marshal(updateGroup)
	require.NoError(err)

	_, res, err = suite.ServeRequest(
		http.MethodPatch,
		"/organizations/:organization/security-groups/:id", fmt.Sprintf("/organizations/%s/security-groups/%s", suite.testUserID.String(), actualGroup.ID),
		func(c *gin.Context) {
			c.Set("nexodus.secGroupsEnabled", "true")
			suite.api.UpdateSecurityGroup(c)
		},
		bytes.NewBuffer(updateBody),
	)
	require.NoError(err)

	// Should be http.StatusStatusUnprocessableEntity.
	require.Equal(http.StatusUnprocessableEntity, res.Code)
}
