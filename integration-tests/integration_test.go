//go:build integration
// +build integration

package integration_tests

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/redhat-et/apex/internal/apex"
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
}

func TestApexIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ApexIntegrationSuite))
}

func (suite *ApexIntegrationSuite) TestBasicConnectivity() {
	assert := suite.Assert()
	require := suite.Require()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := getToken(ctx, "admin@apex.local", "floofykittens")
	require.NoError(err)

	// create the nodes
	node1 := suite.CreateNode("node1", "podman", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", "podman", []string{})
	defer node2.Close()

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

	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)

	suite.T().Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	require.NoError(err)

	suite.T().Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	require.NoError(err)

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
	newNode1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	newNode2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := getToken(ctx, "admin@apex.local", "floofykittens")
	require.NoError(err)

	// create the nodes
	node1 := suite.CreateNode("node1", "podman", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", "podman", []string{})
	defer node2.Close()

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := getToken(ctx, "kitteh1@apex.local", "floofykittens")
	require.NoError(err)

	c, err := newClient(ctx, token)
	require.NoError(err)
	// create a new zone
	zoneID, err := c.CreateZone("zone-blue", "zone full of blue things", "10.140.0.0/24", false)
	require.NoError(err)

	// patch the new user into the zone
	_, err = c.MoveCurrentUserToZone(zoneID.ID)
	require.NoError(err)

	node1IP := "10.140.0.101"
	node2IP := "10.140.0.102"

	// create the nodes
	node1 := suite.CreateNode("node1", "podman", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", "podman", []string{})
	defer node2.Close()

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
	suite.T().Log("Restarting Apex on two spoke nodes and re-joining")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := getToken(ctx, "kitteh2@apex.local", "floofykittens")
	require.NoError(err)

	c, err := newClient(ctx, token)
	require.NoError(err)

	// create a new zone
	zoneID, err := c.CreateZone("zone-relay", "zone with a relay hub", "10.162.0.0/24", true)
	require.NoError(err)

	// patch the new user into the zone
	_, err = c.MoveCurrentUserToZone(zoneID.ID)
	require.NoError(err)

	// create the nodes
	node1 := suite.CreateNode("node1", "podman", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", "podman", []string{})
	defer node2.Close()
	node3 := suite.CreateNode("node3", "podman", []string{})
	defer node2.Close()

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

	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)
	node3IP, err := getContainerIfaceIP(ctx, "wg0", node3)
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := getToken(ctx, "kitteh3@apex.local", "floofykittens")
	require.NoError(err)

	c, err := newClient(ctx, token)
	require.NoError(err)

	// create a new zone
	zoneID, err := c.CreateZone("zone-child-prefix", "zone full of toddler prefixes", "100.64.100.0/24", false)
	require.NoError(err)

	// patch the new user into the zone
	_, err = c.MoveCurrentUserToZone(zoneID.ID)
	require.NoError(err)

	node1LoopbackNet := "172.16.10.101/32"
	node2LoopbackNet := "172.16.20.102/32"
	node1ChildPrefix := "172.16.10.0/24"
	node2ChildPrefix := "172.16.20.0/24"

	// create the nodes
	node1 := suite.CreateNode("node1", "podman", []string{})
	defer node1.Close()
	node2 := suite.CreateNode("node2", "podman", []string{})
	defer node2.Close()

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
	require.NoError(err)
	_, err = containerExec(ctx, node2, []string{"ip", "addr", "add", node2LoopbackNet, "dev", "lo"})
	require.NoError(err)

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

/*
The following test sets up a NAT scenario that emulates
two networks that are behind  NAT devices and validates
connectivity between all nodes.

Spoke nodes within the same network should peer directly
to one another and spoke nodes that are not directly
reachable through their local addresses get relayed
through the relay node.

	           +----------+
	           |  Relay   |
	           |  Node    |
	           +-+------+-+
	  +----------+      +----------+
	  |                            |
	  |                            |
    +-----------+                 +-----------+
    |   NAT     |                 |   NAT     |
    |   Router  |                 |   Router  |
    ++---------++                 ++---------++
     |         |                   |        |
     |         |                   |        |
+-----+---+   ++--------+   +------+--+   +-+-------+
|  Net1   |   |  Net1   |   |  Net2   |   |  Net2   |
|  Spoke1 |   |  Spoke2 |   |  Spoke1 |   |  Spoke2 |
+---------+   +---------+   +---------+   +---------+
*/
// TestRelayNAT tests end to end and spoke to spoke in an easy NAT environment
func (suite *ApexIntegrationSuite) TestRelayNAT() {
	assert := suite.Assert()
	require := suite.Require()

	net1 := "net1"
	net2 := "net2"
	defaultNSNet := "podman"
	docker0 := "172.17.0.1"
	relayNodeName := "relay"
	net1Spoke1Name := "net1-spoke1"
	net2Spoke1Name := "net2-spoke1"
	net1Spoke2Name := "net1-spoke2"
	net2Spoke2Name := "net2-spoke2"
	controllerURL := "http://172.17.0.1:8080"

	// launch a relay node in the default namespace that all spokes can reach
	relayNode := suite.CreateNode(relayNodeName, defaultNSNet, []string{})
	defer relayNode.Close()

	_ = suite.CreateNetwork("net1", "100.64.11.0/24")
	_ = suite.CreateNetwork("net2", "100.64.12.0/24")

	// launch nat nodes
	natNodeNet1 := suite.CreateNode("net1-nat", net1, []string{})
	defer natNodeNet1.Close()
	natNodeNet2 := suite.CreateNode("net2-nat", net2, []string{})
	defer natNodeNet2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// attach nat nodes to the spoke networks
	_, err := apex.RunCommand("docker", "network", "connect", defaultNSNet, "net1-nat")
	require.NoError(err)
	_, err = apex.RunCommand("docker", "network", "connect", defaultNSNet, "net2-nat")
	require.NoError(err)

	// register the nat node interfaces which will be the gateways for spoke nodes
	gatewayNet1, err := getContainerIfaceIP(ctx, "eth0", natNodeNet1)
	require.NoError(err)
	gatewayNet2, err := getContainerIfaceIP(ctx, "eth0", natNodeNet2)
	require.NoError(err)

	// enable masquerading on the nat nodes
	_, err = containerExec(ctx, natNodeNet1, []string{"iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth1", "-j", "MASQUERADE"})
	require.NoError(err)
	_, err = containerExec(ctx, natNodeNet2, []string{"iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth1", "-j", "MASQUERADE"})
	require.NoError(err)

	// create spoke nodes
	net1SpokeNode1 := suite.CreateNode(net1Spoke1Name, net1, []string{})
	defer natNodeNet1.Close()
	net2SpokeNode1 := suite.CreateNode(net2Spoke1Name, net2, []string{})
	defer natNodeNet2.Close()
	net1SpokeNode2 := suite.CreateNode(net1Spoke2Name, net1, []string{})
	defer natNodeNet1.Close()
	net2SpokeNode2 := suite.CreateNode(net2Spoke2Name, net2, []string{})
	defer natNodeNet2.Close()

	// delete the default route pointing to the nat gateway
	_, err = containerExec(ctx, net1SpokeNode1, []string{"ip", "-4", "route", "del", "default"})
	require.NoError(err)
	_, err = containerExec(ctx, net2SpokeNode1, []string{"ip", "-4", "route", "del", "default"})
	require.NoError(err)
	_, err = containerExec(ctx, net1SpokeNode1, []string{"ip", "-4", "route", "add", "default", "via", gatewayNet1})
	require.NoError(err)
	_, err = containerExec(ctx, net2SpokeNode1, []string{"ip", "-4", "route", "add", "default", "via", gatewayNet2})
	require.NoError(err)
	_, err = containerExec(ctx, net1SpokeNode2, []string{"ip", "-4", "route", "del", "default"})
	require.NoError(err)
	_, err = containerExec(ctx, net2SpokeNode2, []string{"ip", "-4", "route", "del", "default"})
	require.NoError(err)
	_, err = containerExec(ctx, net1SpokeNode2, []string{"ip", "-4", "route", "add", "default", "via", gatewayNet1})
	require.NoError(err)
	_, err = containerExec(ctx, net2SpokeNode2, []string{"ip", "-4", "route", "add", "default", "via", gatewayNet2})
	require.NoError(err)

	suite.T().Logf("Validate NAT Infra: Pinging %s from net1-spoke1", docker0)
	err = ping(ctx, net1SpokeNode1, docker0)
	assert.NoError(err)
	suite.T().Logf("Validate NAT Infra: Pinging %s from net2-spoke1", docker0)
	err = ping(ctx, net2SpokeNode1, docker0)
	assert.NoError(err)
	suite.T().Logf("Validate NAT Infra: Pinging %s from net1-spoke1", docker0)
	err = ping(ctx, net1SpokeNode2, docker0)
	assert.NoError(err)
	suite.T().Logf("Validate NAT Infra: Pinging %s from net2-spoke1", docker0)
	err = ping(ctx, net2SpokeNode2, docker0)
	assert.NoError(err)

	token, err := getToken(ctx, "kitteh4@apex.local", "floofykittens")
	require.NoError(err)

	c, err := newClient(ctx, token)
	require.NoError(err)

	// create a new zone
	zoneID, err := c.CreateZone("zone-nat-relay", "nat test zone", "10.29.0.0/24", true)
	require.NoError(err)

	// patch the new user into the zone
	_, err = c.MoveCurrentUserToZone(zoneID.ID)
	require.NoError(err)

	// start apex on the nodes
	go func() {
		_, err = containerExec(ctx, relayNode, []string{
			"/bin/apex",
			"--hub-router",
			fmt.Sprintf("--with-token=%s", token),
			controllerURL,
		})
	}()

	// ensure the relay node has time to register before joining spokes since it is required for hub-zones
	time.Sleep(time.Second * 10)
	go func() {
		_, err = containerExec(ctx, net1SpokeNode1, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			controllerURL,
		})
	}()

	go func() {
		_, err = containerExec(ctx, net2SpokeNode1, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			controllerURL,
		})
	}()

	go func() {
		_, err = containerExec(ctx, net1SpokeNode2, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			controllerURL,
		})
	}()

	go func() {
		_, err = containerExec(ctx, net2SpokeNode2, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			controllerURL,
		})
	}()

	relayNodeIP, err := getContainerIfaceIP(ctx, "wg0", relayNode)
	require.NoError(err)
	net1SpokeNode1IP, err := getContainerIfaceIP(ctx, "wg0", net1SpokeNode1)
	require.NoError(err)
	net2SpokeNode1IP, err := getContainerIfaceIP(ctx, "wg0", net2SpokeNode1)
	require.NoError(err)
	net1SpokeNode2IP, err := getContainerIfaceIP(ctx, "wg0", net1SpokeNode2)
	require.NoError(err)
	net2SpokeNode2IP, err := getContainerIfaceIP(ctx, "wg0", net2SpokeNode2)
	require.NoError(err)

	suite.T().Logf("Pinging %s %s from %s", net1Spoke1Name, net1SpokeNode1IP, relayNodeName)
	err = ping(ctx, relayNode, net1SpokeNode1IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from %s", net2Spoke1Name, net2SpokeNode1IP, relayNodeName)
	err = ping(ctx, relayNode, net2SpokeNode1IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", net1Spoke1Name, net1SpokeNode1IP, net2Spoke1Name)
	err = ping(ctx, net2SpokeNode1, net1SpokeNode1IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", net2Spoke1Name, net2SpokeNode1IP, net1Spoke1Name)
	err = ping(ctx, net1SpokeNode1, net2SpokeNode1IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", relayNodeName, relayNodeIP, net1Spoke1Name)
	err = ping(ctx, net1SpokeNode1, relayNodeIP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", relayNodeName, relayNodeIP, net2Spoke1Name)
	err = ping(ctx, net2SpokeNode1, relayNodeIP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", relayNodeName, relayNodeIP, net1Spoke2Name)
	err = ping(ctx, net1SpokeNode2, relayNodeIP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", relayNodeName, relayNodeIP, net2Spoke2Name)
	err = ping(ctx, net2SpokeNode2, relayNodeIP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", net1Spoke1Name, net1SpokeNode1IP, net1Spoke2Name)
	err = ping(ctx, net1SpokeNode2, net1SpokeNode1IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", net2Spoke1Name, net2SpokeNode1IP, net2Spoke2Name)
	err = ping(ctx, net2SpokeNode2, net2SpokeNode1IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", net2Spoke2Name, net2SpokeNode2IP, net1Spoke2Name)
	err = ping(ctx, net1SpokeNode2, net2SpokeNode2IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", net1Spoke2Name, net1SpokeNode2IP, net2Spoke1Name)
	err = ping(ctx, net2SpokeNode1, net1SpokeNode2IP)
	assert.NoError(err)
	// dump the wg state from the relay node. Likely temporary but
	// important to show what is being tested here by displaying
	// sockets being opened from an outside NAT interface.
	wgShow, err := containerExec(ctx, relayNode, []string{"wg", "show", "wg0", "dump"})
	require.NoError(err)
	suite.T().Logf("Relay node wireguard state: \n%s", wgShow)

	// kill the apex process on both nodes
	_, err = containerExec(ctx, net1SpokeNode1, []string{"killall", "apex"})
	require.NoError(err)
	_, err = containerExec(ctx, net2SpokeNode1, []string{"killall", "apex"})
	require.NoError(err)

	// restart the process on two nodes and verify re-joining
	suite.T().Log("Restarting apex on two spoke nodes and re-joining")
	go func() {
		_, err = containerExec(ctx, net1SpokeNode1, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			controllerURL,
		})
	}()

	go func() {
		_, err = containerExec(ctx, net2SpokeNode1, []string{
			"/bin/apex",
			fmt.Sprintf("--with-token=%s", token),
			controllerURL,
		})
	}()

	// validate the re-joined nodes can communicate
	suite.T().Logf("Pinging %s %s from node %s", net2Spoke1Name, net2SpokeNode1IP, net1Spoke1Name)
	err = ping(ctx, net1SpokeNode1, net2SpokeNode1IP)
	assert.NoError(err)

	suite.T().Logf("Pinging %s %s from node %s", net1Spoke1Name, net1SpokeNode1IP, net2Spoke1Name)
	err = ping(ctx, net2SpokeNode1, net1SpokeNode1IP)
	assert.NoError(err)

	// verify there are (n) lines in the wg show output on a spoke node in each network
	wgSpokeShow, err := containerExec(ctx, net1SpokeNode1, []string{"wg", "show", "wg0", "dump"})
	require.NoError(err)
	lc, err := lineCount(wgSpokeShow)
	require.NoError(err)
	assert.Equal(3, lc, "the number of expected wg show peers was %d, found %d: wg show out: \n%s", 3, lc, wgSpokeShow)

	// verify there are (n) lines in the wg show output on a spoke node in each network
	wgSpokeShow, err = containerExec(ctx, net2SpokeNode1, []string{"wg", "show", "wg0", "dump"})
	require.NoError(err)
	lc, err = lineCount(wgSpokeShow)
	require.NoError(err)
	assert.Equal(3, lc, "the number of expected wg show peers was %d, found %d: wg show out: \n%s", 3, lc, wgSpokeShow)
}
