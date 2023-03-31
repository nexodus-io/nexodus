//go:build integration
// +build integration

package integration_tests

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/cenkalti/backoff/v4"
	"github.com/cucumber/godog"
	"github.com/nexodus-io/nexodus/internal/cucumber"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

var providerType testcontainers.ProviderType
var defaultNetwork string
var hostDNSName string
var ipamDriver string

const nexctl = "../dist/nexctl"

func init() {
	if os.Getenv("NEXODUS_TEST_PODMAN") != "" {
		fmt.Println("Using podman")
		providerType = testcontainers.ProviderPodman
		defaultNetwork = "podman"
		ipamDriver = "host-local"
		hostDNSName = "10.88.0.1"
	} else {
		fmt.Println("Using docker")
		providerType = testcontainers.ProviderDocker
		defaultNetwork = "bridge"
		ipamDriver = "default"
		hostDNSName = dockerKindGatewayIP()
	}
	_ = nexodus.CreateDirectory("tmp")
}

func dockerKindGatewayIP() string {
	ip := nexodus.LocalIPv4Address()
	if ip == nil {
		panic("local ip address not found")
	}
	return ip.String()
}

type NexodusIntegrationSuite struct {
	suite.Suite
	logger  *zap.SugaredLogger
	gocloak *gocloak.GoCloak
}

func (suite *NexodusIntegrationSuite) Context() context.Context {
	return context.WithValue(context.Background(), "suite", suite)
}

func GetNexodusIntegrationSuite(ctx context.Context) *NexodusIntegrationSuite {
	if ctx == nil {
		return nil
	}
	if rc, ok := ctx.Value("suite").(*NexodusIntegrationSuite); ok {
		return rc
	}
	return nil
}
func TestNexodusIntegrationSuite(t *testing.T) {
	suite.Run(t, new(NexodusIntegrationSuite))
}

func (suite *NexodusIntegrationSuite) SetupSuite() {
	logger := zaptest.NewLogger(suite.T())
	suite.logger = logger.Sugar()
	suite.gocloak = gocloak.NewClient("https://auth.try.nexodus.127.0.0.1.nip.io")
	suite.gocloak.RestyClient().SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
}

func (suite *NexodusIntegrationSuite) TestBasicConnectivity() {
	suite.T().Parallel()
	assert := suite.Assert()
	require := suite.Require()
	parentCtx := suite.Context()
	ctx, cancel := context.WithTimeout(parentCtx, 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username := suite.createNewUser(ctx, password)

	// create the nodes
	node1 := suite.CreateNode(ctx, "TestBasicConnectivity-node1", []string{defaultNetwork})
	node2 := suite.CreateNode(ctx, "TestBasicConnectivity-node2", []string{defaultNetwork})

	// start nexodus on the nodes
	suite.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2, "--username", username, "--password", password)

	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)

	gather := suite.gatherFail(ctx, node1, node2)
	suite.logger.Infof("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoErrorf(err, gather)

	suite.logger.Info("killing nexodus and re-joining nodes with new keys")
	//kill the nexodus process on both nodes
	_, err = suite.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = suite.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)

	// delete only the public key on node1
	_, err = suite.containerExec(ctx, node1, []string{"rm", "/etc/wireguard/public.key"})
	require.NoError(err)
	// delete the entire wireguard directory on node2
	_, err = suite.containerExec(ctx, node2, []string{"rm", "-rf", "/etc/wireguard/"})
	require.NoError(err)

	// start nexodus on the nodes
	go suite.runNexd(ctx, node1, "--username", username, "--password", password)
	go suite.runNexd(ctx, node2, "--username", username, "--password", password)

	var newNode1IP string
	err = backoff.Retry(
		func() error {
			var err error
			newNode1IP, err = getContainerIfaceIP(ctx, "wg0", node1)
			if err != nil {
				return err
			}
			if newNode1IP == node1IP {
				return fmt.Errorf("new node1IP is the same as old ip")
			}
			return nil
		},
		backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx),
	)
	require.NoError(err)

	var newNode2IP string
	err = backoff.Retry(
		func() error {
			var err error
			newNode2IP, err = getContainerIfaceIP(ctx, "wg0", node2)
			if err != nil {
				return err
			}
			if newNode2IP == node2IP {
				return fmt.Errorf("new node1IP is the same as old ip")
			}
			return nil
		},
		backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx),
	)
	require.NoError(err)

	gather = suite.gatherFail(ctx, node1, node2)
	suite.logger.Infof("Pinging %s from node1", newNode2IP)
	err = ping(ctx, node1, newNode2IP)
	assert.NoError(err, gather)

	suite.logger.Infof("Pinging %s from node2", newNode1IP)
	err = ping(ctx, node2, newNode1IP)
	assert.NoErrorf(err, gather)
}

// TestRequestIPOrganization tests requesting a specific address in a newly created organization
func (suite *NexodusIntegrationSuite) TestRequestIPOrganization() {
	suite.T().Parallel()
	assert := suite.Assert()
	require := suite.Require()
	parentCtx := suite.Context()
	ctx, cancel := context.WithTimeout(parentCtx, 120*time.Second)
	defer cancel()
	password := "floofykittens"
	username := suite.createNewUser(ctx, password)
	node2IP := "100.100.0.102"

	// create the nodes
	node1 := suite.CreateNode(ctx, "TestRequestIPOrganization-node1", []string{defaultNetwork})
	node2 := suite.CreateNode(ctx, "TestRequestIPOrganization-node2", []string{defaultNetwork})

	// start nexodus on the nodes
	suite.runNexd(ctx, node1, "--discovery-node", "--relay-node",
		"--username", username, "--password", password)

	// validate nexd has started on the discovery node
	err := suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2,
		"--username", username, "--password", password,
		fmt.Sprintf("--request-ip=%s", node2IP),
	)

	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)

	gather := suite.gatherFail(ctx, node1, node2)

	// ping the requested IP address (--request-ip)
	suite.logger.Infof("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoErrorf(err, gather)

	suite.logger.Info("killing nexodus and re-joining nodes")
	//kill the nexodus process on both nodes
	_, err = suite.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = suite.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)

	// restart nexodus and ensure the nodes receive the same re-quested address
	suite.logger.Info("Restarting nexodus on two spoke nodes and re-joining")
	suite.runNexd(ctx, node1, "--discovery-node", "--relay-node",
		"--username", username, "--password", password,
		fmt.Sprintf("--request-ip=%s", node1IP),
	)

	// validate nexd has started on the discovery node
	err = suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2,
		"--username", username, "--password", password,
		fmt.Sprintf("--request-ip=%s", node2IP),
	)

	gather = suite.gatherFail(ctx, node1, node2)

	// ping the requested IP address (--request-ip)
	suite.logger.Infof("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoErrorf(err, gather)
}

// TestHubOrganization test a hub organization with 3 nodes, the first being a relay node
func (suite *NexodusIntegrationSuite) TestHubOrganization() {
	suite.T().Parallel()
	assert := suite.Assert()
	require := suite.Require()
	parentCtx := suite.Context()
	ctx, cancel := context.WithTimeout(parentCtx, 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username := suite.createNewUser(ctx, password)

	// create the nodes
	node1 := suite.CreateNode(ctx, "TestHubOrganization-node1", []string{defaultNetwork})
	node2 := suite.CreateNode(ctx, "TestHubOrganization-node2", []string{defaultNetwork})
	node3 := suite.CreateNode(ctx, "TestHubOrganization-node3", []string{defaultNetwork})

	// start nexodus on the nodes
	suite.runNexd(ctx, node1, "--discovery-node", "--relay-node", "--username", username, "--password", password)

	// validate nexd has started on the discovery node
	err := suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2, "--username", username, "--password", password)
	suite.runNexd(ctx, node3, "--username", username, "--password", password)

	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)
	node3IP, err := getContainerIfaceIP(ctx, "wg0", node3)
	require.NoError(err)

	gather := suite.gatherFail(ctx, node1, node2, node3)

	suite.logger.Infof("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node1", node3IP)
	err = ping(ctx, node1, node3IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node3", node1IP)
	err = ping(ctx, node3, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, node3IP)
	assert.NoErrorf(err, gather)

	hubOrganizationChildPrefix := "10.188.100.0/24"
	node2ChildPrefixLoopbackNet := "10.188.100.1/32"

	suite.T().Logf("killing nexodus on node2")

	_, err = suite.containerExec(ctx, node2, []string{"killall", "nexd"})
	assert.NoError(err)
	suite.T().Logf("rejoining on node2 with --child-prefix=%s", hubOrganizationChildPrefix)

	// add a loopback that are contained in the node's child prefix
	_, err = suite.containerExec(ctx, node2, []string{"ip", "addr", "add", node2ChildPrefixLoopbackNet, "dev", "lo"})
	require.NoError(err)

	suite.runNexd(ctx, node2, "--username", username, "--password", password,
		fmt.Sprintf("--child-prefix=%s", hubOrganizationChildPrefix),
	)

	// validate nexd has started on the discovery node
	err = suite.nexdStatus(ctx, node2)
	require.NoError(err)

	// address will be the same, this is just a readiness check for gather data
	node1IP, err = getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err = getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)
	node3IP, err = getContainerIfaceIP(ctx, "wg0", node3)
	require.NoError(err)

	gather = suite.gatherFail(ctx, node1, node2, node3)

	// parse the loopback ip from the loopback prefix
	node2LoopbackIP, _, _ := net.ParseCIDR(node2ChildPrefixLoopbackNet)

	suite.T().Logf("Pinging loopback on node2 %s from node3 wg0", node2LoopbackIP.String())
	err = ping(ctx, node3, node2LoopbackIP.String())
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node1", node3IP)
	err = ping(ctx, node1, node3IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node3", node1IP)
	err = ping(ctx, node3, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, node3IP)
	assert.NoErrorf(err, gather)

	// get the device id for node3
	userOut, err := suite.runCommand(nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"user", "get-current",
	)
	require.NoErrorf(err, "nexctl user list error: %v\n", err)
	var user models.UserJSON
	err = json.Unmarshal([]byte(userOut), &user)
	assert.Equal(1, len(user.Organizations))
	orgID := user.Organizations[0]

	allDevices, err := suite.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json-raw",
		"device", "list", "--organization-id", orgID.String(),
	)
	var devices []models.Device
	json.Unmarshal([]byte(allDevices), &devices)
	assert.NoErrorf(err, "nexctl device list error: %v\n", err)

	// register node3 device ID for node3 for deletion
	var device3ID string
	node3Hostname, err := suite.getNodeHostname(ctx, node3)
	suite.logger.Infof("deleting node3 running in container: %s", node3Hostname)
	assert.NoError(err)
	for _, p := range devices {
		if p.Hostname == node3Hostname {
			node3IP = p.TunnelIP
			device3ID = p.ID.String()
		}
	}

	// delete the device node2
	_, err = suite.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"device", "delete",
		"--device-id", device3ID,
	)
	require.NoError(err)

	// this is probably more time than needed for convergence as polling is currently 5s
	time.Sleep(time.Second * 10)
	gather = suite.gatherFail(ctx, node1, node2, node3)

	// verify the deleted device details are no longer in a device's tables
	node2routes := suite.routesDump(ctx, node2)
	node2dump := suite.wgDump(ctx, node2)

	assert.NotContainsf(node2routes, node3IP, "found deleted device node still in routing tables of a device", gather)
	assert.NotContainsf(node2dump, node3IP, "found deleted device node still in wg show wg0 dump tables of a device", gather)
}

// TestChildPrefix tests requesting a specific address in a newly created organization
func (suite *NexodusIntegrationSuite) TestChildPrefix() {
	suite.T().Parallel()
	assert := suite.Assert()
	require := suite.Require()
	parentCtx := suite.Context()
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username := suite.createNewUser(ctx, password)
	node1LoopbackNet := "172.16.10.101/32"
	node2LoopbackNet := "172.16.20.102/32"
	node1ChildPrefix := "172.16.10.0/24"
	node2ChildPrefix := "172.16.20.0/24"

	// create the nodes
	node1 := suite.CreateNode(ctx, "TestChildPrefix-node1", []string{defaultNetwork})
	node2 := suite.CreateNode(ctx, "TestChildPrefix-node2", []string{defaultNetwork})

	// start nexodus on the nodes
	suite.runNexd(ctx, node1, "--discovery-node", "--relay-node",
		fmt.Sprintf("--child-prefix=%s", node1ChildPrefix),
		"--username", username, "--password", password,
	)

	// validate nexd has started on the discovery node
	err := suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2,
		fmt.Sprintf("--child-prefix=%s", node2ChildPrefix),
		"--username", username, "--password", password,
	)

	// add loopbacks to the containers that are contained in the node's child prefix
	_, err = suite.containerExec(ctx, node1, []string{"ip", "addr", "add", node1LoopbackNet, "dev", "lo"})
	require.NoError(err)
	_, err = suite.containerExec(ctx, node2, []string{"ip", "addr", "add", node2LoopbackNet, "dev", "lo"})
	require.NoError(err)

	// parse the loopback ip from the loopback prefix
	node1LoopbackIP, _, _ := net.ParseCIDR(node1LoopbackNet)
	node2LoopbackIP, _, _ := net.ParseCIDR(node2LoopbackNet)

	// address will be the same, this is just a readiness check for gather data
	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)

	gather := suite.gatherFail(ctx, node1, node2)

	suite.logger.Infof("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node1", node2LoopbackIP)
	err = ping(ctx, node1, node2LoopbackIP.String())
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node1LoopbackIP)
	err = ping(ctx, node2, node1LoopbackIP.String())
	assert.NoErrorf(err, gather)
}

// TestRelay validates the scenario where the agent is set to explicitly relay only.
func (suite *NexodusIntegrationSuite) TestRelay() {
	suite.T().Parallel()
	assert := suite.Assert()
	require := suite.Require()
	parentCtx := suite.Context()
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username := suite.createNewUser(ctx, password)

	// create the nodes
	node1 := suite.CreateNode(ctx, "TestRelay-node1", []string{defaultNetwork})
	node2 := suite.CreateNode(ctx, "TestRelay-node2", []string{defaultNetwork})
	node3 := suite.CreateNode(ctx, "TestRelay-node3", []string{defaultNetwork})

	// start nexodus on the nodes
	suite.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2, "--username", username, "--password", password)
	suite.runNexd(ctx, node3, "--username", username, "--password", password, "--relay-only")

	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)
	node3IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)

	gather := suite.gatherFail(ctx, node1, node2, node3)
	suite.logger.Infof("Pinging %s from node1", node3IP)
	err = ping(ctx, node1, node3IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, node3IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node3", node1IP)
	err = ping(ctx, node3, node1IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node3", node2IP)
	err = ping(ctx, node3, node2IP)
	assert.NoErrorf(err, gather)
}

func (suite *NexodusIntegrationSuite) Testnexctl() {
	suite.T().Parallel()
	assert := suite.Assert()
	require := suite.Require()
	parentCtx := suite.Context()
	ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
	defer cancel()
	password := "floofykittens"
	username := suite.createNewUser(ctx, password)

	// create the nodes
	node1 := suite.CreateNode(ctx, "Testnexctl-node1", []string{defaultNetwork})
	node2 := suite.CreateNode(ctx, "Testnexctl-node2", []string{defaultNetwork})

	// validate nexctl user get-current returns a user
	userOut, err := suite.runCommand(nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"user", "get-current",
	)
	require.NoErrorf(err, "nexctl user list error: %v\n", err)
	var user models.UserJSON
	err = json.Unmarshal([]byte(userOut), &user)
	assert.NotEmpty(user)
	require.NotEmpty(user.UserName)
	require.Equal(1, len(user.Organizations))

	// start nexodus on the nodes
	suite.runNexd(ctx, node1, "--discovery-node", "--relay-node", "--username", username, "--password", password)

	// validate nexd has started on the discovery node
	err = suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2, "--username", username, "--password", password, "--child-prefix=100.22.100.0/24")

	node1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, "wg0", node2)
	require.NoError(err)

	gather := suite.gatherFail(ctx, node1, node2)
	suite.logger.Infof("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, node2IP)
	assert.NoErrorf(err, gather)

	suite.logger.Infof("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, node1IP)
	assert.NoErrorf(err, gather)

	// validate list devices and register IDs and IPs
	allDevices, err := suite.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json-raw",
		"device", "list",
	)
	var devices []models.Device
	json.Unmarshal([]byte(allDevices), &devices)
	assert.NoErrorf(err, "nexctl device list error: %v\n", err)

	// register the device IDs for node1 and node2 for deletion
	var node1DeviceID string
	var node2DeviceID string
	node1Hostname, err := suite.getNodeHostname(ctx, node1)
	suite.logger.Infof("deleting Node1 running in container: %s", node1Hostname)
	assert.NoError(err)
	node2Hostname, err := suite.getNodeHostname(ctx, node2)
	suite.logger.Infof("deleting Node2 running in container: %s", node2Hostname)
	assert.NoError(err)

	for _, p := range devices {
		if p.Hostname == node1Hostname {
			node1DeviceID = p.ID.String()
		}
		if p.Hostname == node2Hostname {
			node2DeviceID = p.ID.String()
		}
	}

	//kill the nexodus process on both nodes
	_, err = suite.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = suite.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)

	// delete both devices from nexodus
	_, err = suite.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"device", "delete",
		"--device-id", node1DeviceID,
	)
	require.NoError(err)
	_, err = suite.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"device", "delete",
		"--device-id", node2DeviceID,
	)
	require.NoError(err)

	// delete the keys on both nodes to force ensure the deleted device released it's
	// IPAM address and will re-issue that address to a new device with a new keypair.
	_, err = suite.containerExec(ctx, node1, []string{"rm", "-rf", "/etc/wireguard/"})
	require.NoError(err)
	_, err = suite.containerExec(ctx, node2, []string{"rm", "-rf", "/etc/wireguard/"})
	require.NoError(err)

	time.Sleep(time.Second * 10)
	// re-join both nodes, flipping the child-prefix to node1 to ensure the child-prefix was released
	suite.runNexd(ctx, node1, "--username", username, "--password", password, "--child-prefix=100.22.100.0/24")

	// validate nexd has started on the discovery node
	err = suite.nexdStatus(ctx, node1)
	require.NoError(err)

	suite.runNexd(ctx, node2, "--username", username, "--password", password)

	newNode1IP, err := getContainerIfaceIP(ctx, "wg0", node1)
	require.NoError(err)
	gather = suite.gatherFail(ctx, node1, node2)

	// If the device was not deleted, the next registered device would receive the
	// next available address in the IPAM pool, not the previously assigned address.
	// Fail the test if the device IP was not the previous address from the IPAM pool.
	var addressMatch bool
	if newNode1IP == node2IP {
		addressMatch = true
		suite.logger.Infof("Pinging %s from node1", node1IP)
		err = ping(ctx, node1, node1IP)
		assert.NoErrorf(err, gather)
	}
	if newNode1IP == node1IP {
		addressMatch = true
		suite.logger.Infof("Pinging %s from node1", node2IP)
		err = ping(ctx, node1, node2IP)
		assert.NoErrorf(err, gather)
	}
	if !addressMatch {
		assert.Failf("ipam/device delete failed", fmt.Sprintf("Node did not receive the proper IPAM address %s, it should have been %s or %s\n %s", newNode1IP, node1IP, node2IP, gather))
	}

	// validate list devices in a organization
	devicesInOrganization, err := suite.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json-raw",
		"device", "list",
		"--organization-id", user.Organizations[0].String(),
	)

	json.Unmarshal([]byte(devicesInOrganization), &devices)
	assert.NoErrorf(err, "nexctl device list error: %v\n", err)

	// TODO: this gets resolved with #439 (org delete)
	// re-register the device IDs for node1 and node2 as they have been re-created w/new IDs
	//for _, p := range devices {
	//	if p.TunnelIP == node1IP {
	//		node1DeviceID = p.ID.String()
	//	}
	//	if p.TunnelIP == node2IP {
	//		node2DeviceID = p.ID.String()
	//	}
	//}
	//// delete all devices from the organization as currently required to avoid sql key
	//// constraints, then delete the organization, then recreate the organization to ensure the
	//// IPAM prefix was released. If it was not released the creation will fail.
	//_, err = suite.runCommand(nexctl,
	//	"--username", username,
	//	"--password", password,
	//	"device", "delete",
	//	"--device-id", node1DeviceID,
	//)
	//require.NoError(err)
	//
	//_, err = suite.runCommand(nexctl,
	//	"--username", username,
	//	"--password", password,
	//	"device", "delete",
	//	"--device-id", node2DeviceID,
	//)
	//require.NoError(err)
	//
	//// delete the organization
	//_, err = suite.runCommand(nexctl,
	//	"--username", username,
	//	"--password", password,
	//	"--output", "json",
	//	"organization", "delete",
	//	"--organization-id", user.Organizations[0].String(),
	//)
	//require.NoError(err)
	//
	//// re-create the deleted organization, this will fail if the IPAM
	//// prefix was not released from the prior deletion
	//_, err = suite.runCommand(nexctl,
	//	"--username", username,
	//	"--password", password,
	//	"organization", "create",
	//	"--name", "kitteh5",
	//	"--cidr", "100.100.1.0/20",
	//	"--description", "kitteh5's organization",
	//)
	//require.NoError(err)
}

func (suite *NexodusIntegrationSuite) TestFeatures() {

	// This looks for feature files in the current directory
	var cucumberOptions = cucumber.DefaultOptions()
	// configures where to look for feature files.
	cucumberOptions.Paths = []string{"."}
	// output more info when test is run in verbose mode.
	for _, arg := range os.Args[1:] {
		if arg == "-test.v=true" || arg == "-test.v" || arg == "-v" { // go test transforms -v option
			cucumberOptions.Format = "pretty"
		}
	}

	tlsConfig := suite.NewTLSConfig()

	for i := range cucumberOptions.Paths {
		root := cucumberOptions.Paths[i]

		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {

			suite.Require().NoError(err)

			if info.IsDir() {
				return nil
			}

			name := filepath.Base(info.Name())
			ext := filepath.Ext(info.Name())

			if ext != ".feature" {
				return nil
			}

			suite.T().Run(name, func(t *testing.T) {

				// To preserve the current behavior, the test are market to be "safely" run in parallel, however
				// we may think to introduce a new naming convention i.e. files that ends with _parallel would
				// cause t.Parallel() to be invoked, other tests won't, so they won't be executed concurrently.
				//
				// This could help reducing/removing the need of explicit lock
				t.Parallel()

				o := cucumberOptions
				o.TestingT = t
				o.Paths = []string{path.Join(root, name)}

				s := cucumber.NewTestSuite()
				s.Context = suite.Context()
				s.ApiURL = "https://api.try.nexodus.127.0.0.1.nip.io"
				s.TlsConfig = tlsConfig

				status := godog.TestSuite{
					Name:                name,
					Options:             &o,
					ScenarioInitializer: s.InitializeScenario,
				}.Run()
				if status != 0 {
					suite.T().Fail()
				}
			})
			return nil
		})
		suite.Require().NoError(err)
	}
}
