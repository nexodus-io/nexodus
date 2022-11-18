//go:build integration
// +build integration

package integration_tests

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ApexIntegrationSuite struct {
	suite.Suite
	pool *dockertest.Pool
}

func (suite *ApexIntegrationSuite) SetupSuite() {
	var err error
	suite.pool, err = dockertest.NewPool("")
	require.NoError(suite.T(), err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	err = backoff.Retry(healthcheck, backoff.WithContext(backoff.NewExponentialBackOff(), ctx))
	require.NoError(suite.T(), err)
}

func TestApexIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ApexIntegrationSuite))
}

func (suite *ApexIntegrationSuite) TestBasicConnectivity() {
	assert := suite.Assert()
	require := suite.Require()

	token, err := GetToken("admin", "floofykittens")
	require.NoError(err)

	// create the nodes
	node1 := suite.CreateNode("node1", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", []string{})
	defer node2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// start apex on the nodes
	go func() {
		_, err = containerExec(ctx, node1, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	go func() {
		_, err = containerExec(ctx, node2, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	node1IP, err := getWg0IP(ctx, node1)
	require.NoError(err)
	node2IP, err := getWg0IP(ctx, node2)
	require.NoError(err)

	suite.T().Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoError(err)

	//kill the apex process on both nodes
	_, err = containerExec(ctx, node1, []string{"killall", "apex"})
	require.NoError(err)
	_, err = containerExec(ctx, node2, []string{"killall", "apex"})
	require.NoError(err)

	// delete only the public key on node1
	_, err = containerExec(ctx, node1, []string{"rm", "/etc/wireguard/public.key"})
	require.NoError(err)
	// delete the entire wireguard directory on node2
	_, err = containerExec(ctx, node2, []string{"rm", "-rf", "/etc/wireguard/"})
	require.NoError(err)

	// start apex on the nodes
	go func() {
		_, err = containerExec(ctx, node1, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	go func() {
		_, err = containerExec(ctx, node2, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	// give wg0 time to re-address
	time.Sleep(time.Second * 5)
	// IPs will be new since the keys were deleted, retrieve the new addresses
	newNode1IP, err := getWg0IP(ctx, node1)
	require.NoError(err)
	newNode2IP, err := getWg0IP(ctx, node2)
	require.NoError(err)

	suite.T().Logf("Pinging %s from node1", newNode2IP)
	err = ping(ctx, node1, newNode2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", newNode1IP)
	err = ping(ctx, node2, newNode1IP)
	assert.NoError(err)
}

// TestRequestIPDefaultZone tests requesting a specific address in the default zone
func (suite *ApexIntegrationSuite) TestRequestIPDefaultZone() {
	assert := suite.Assert()
	require := suite.Require()

	node1IP := "10.200.0.101"
	node2IP := "10.200.0.102"
	token, err := GetToken("admin", "floofykittens")
	require.NoError(err)

	// create the nodes
	node1 := suite.CreateNode("node1", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", []string{})
	defer node2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// start apex on the nodes
	go func() {
		_, err = containerExec(ctx, node1, []string{
			"/bin/apex",
			fmt.Sprintf("--request-ip=%s", node1IP),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	go func() {
		_, err = containerExec(ctx, node2, []string{
			"/bin/apex",
			fmt.Sprintf("--request-ip=%s", node2IP),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	// ping the requested IP address (--request-ip)
	suite.T().Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoError(err)
}

// TestRequestIPZone tests requesting a specific address in a newly created zone
func (suite *ApexIntegrationSuite) TestRequestIPZone() {
	assert := suite.Assert()
	require := suite.Require()
	token, err := GetToken("kitteh1", "floofykittens")
	require.NoError(err)

	c, err := newClient(token)
	require.NoError(err)
	// create a new zone
	zoneID, err := c.CreateZone("zone-blue", "zone full of blue things", "10.140.0.0/24", false)
	assert.NoError(err)

	// patch the new user into the zone
	_, err = c.MoveCurrentUserToZone(zoneID.ID)
	assert.NoError(err)

	node1IP := "10.140.0.101"
	node2IP := "10.140.0.102"

	// create the nodes
	node1 := suite.CreateNode("node1", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", []string{})
	defer node2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// start apex on the nodes
	go func() {
		_, err = containerExec(ctx, node1, []string{
			"/bin/apex",
			fmt.Sprintf("--request-ip=%s", node1IP),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	go func() {
		_, err = containerExec(ctx, node2, []string{
			"/bin/apex",
			fmt.Sprintf("--request-ip=%s", node2IP),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	// ping the requested IP address (--request-ip)
	suite.T().Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoError(err)

	//kill the apex process on both nodes
	_, err = containerExec(ctx, node1, []string{"killall", "apex"})
	require.NoError(err)
	_, err = containerExec(ctx, node2, []string{"killall", "apex"})
	require.NoError(err)

	// restart apex and ensure the nodes receive the same re-quested address
	go func() {
		_, err = containerExec(ctx, node1, []string{
			"/bin/apex",
			fmt.Sprintf("--request-ip=%s", node1IP),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	go func() {
		_, err = containerExec(ctx, node2, []string{
			"/bin/apex",
			fmt.Sprintf("--request-ip=%s", node2IP),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	// ping the requested IP address (--request-ip)
	suite.T().Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoError(err)
}

// TestHubZone test a hub zone with 3 nodes, the first being a relay node
func (suite *ApexIntegrationSuite) TestHubZone() {
	assert := suite.Assert()
	require := suite.Require()
	token, err := GetToken("kitteh2", "floofykittens")
	require.NoError(err)

	c, err := newClient(token)
	require.NoError(err)

	// create a new zone
	zoneID, err := c.CreateZone("zone-relay", "zone with a relay hub", "10.162.0.0/24", true)
	assert.NoError(err)

	// patch the new user into the zone
	_, err = c.MoveCurrentUserToZone(zoneID.ID)
	assert.NoError(err)

	// create the nodes
	node1 := suite.CreateNode("node1", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", []string{})
	defer node2.Close()
	node3 := suite.CreateNode("node3", []string{})
	defer node2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// start apex on the nodes
	go func() {
		_, err = containerExec(ctx, node1, []string{
			"/bin/apex",
			"--hub-router",
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	// Ensure the relay node has time to register before joining spokes since it is required for hub-zones
	time.Sleep(time.Second * 10)
	go func() {
		_, err = containerExec(ctx, node2, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	go func() {
		_, err = containerExec(ctx, node3, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	node1IP, err := getWg0IP(ctx, node1)
	require.NoError(err)
	node2IP, err := getWg0IP(ctx, node2)
	require.NoError(err)
	node3IP, err := getWg0IP(ctx, node3)
	require.NoError(err)

	suite.T().Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node1", node3IP)
	err = ping(ctx, node1, node3IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node3", node1IP)
	err = ping(ctx, node3, node2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, node3IP)
	assert.NoError(err)
}

// TestChildPrefix tests requesting a specific address in a newly created zone
func (suite *ApexIntegrationSuite) TestChildPrefix() {
	assert := suite.Assert()
	require := suite.Require()
	token, err := GetToken("kitteh3", "floofykittens")
	require.NoError(err)

	c, err := newClient(token)
	require.NoError(err)

	// create a new zone
	zoneID, err := c.CreateZone("zone-child-prefix", "zone full of toddler prefixes", "100.64.100.0/24", false)
	assert.NoError(err)

	// patch the new user into the zone
	_, err = c.MoveCurrentUserToZone(zoneID.ID)
	assert.NoError(err)

	node1LoopbackNet := "172.16.10.101/32"
	node2LoopbackNet := "172.16.20.102/32"
	node1ChildPrefix := "172.16.10.0/24"
	node2ChildPrefix := "172.16.20.0/24"

	// create the nodes
	node1 := suite.CreateNode("node1", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", []string{})
	defer node2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	// start apex on the nodes
	go func() {
		_, err = containerExec(ctx, node1, []string{
			"/bin/apex",
			fmt.Sprintf("--child-prefix=%s", node1ChildPrefix),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	go func() {
		_, err = containerExec(ctx, node2, []string{
			"/bin/apex",
			fmt.Sprintf("--child-prefix=%s", node2ChildPrefix),
			fmt.Sprintf("--with-token=%s", token),
			"http://host.docker.internal:8080",
		})
	}()

	// add loopbacks to the containers that are contained in the node's child prefix
	_, err = containerExec(ctx, node1, []string{"ip", "addr", "add", node1LoopbackNet, "dev", "lo"})
	assert.NoError(err)
	_, err = containerExec(ctx, node2, []string{"ip", "addr", "add", node2LoopbackNet, "dev", "lo"})
	assert.NoError(err)

	// parse the loopback ip from the loopback prefix
	node1LoopbackIP, _, _ := net.ParseCIDR(node1LoopbackNet)
	node2LoopbackIP, _, _ := net.ParseCIDR(node2LoopbackNet)

	suite.T().Logf("Pinging %s from node1", node2LoopbackIP)
	err = ping(ctx, node1, node2LoopbackIP.String())
	assert.NoError(err)

	suite.T().Logf("Pinging %s from node2", node1LoopbackIP)
	err = ping(ctx, node2, node1LoopbackIP.String())
	assert.NoError(err)
}
