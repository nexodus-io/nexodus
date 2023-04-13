//go:build integration

package integration_tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/nexodus-io/nexodus/internal/util"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestBasicConnectivity(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	node1IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node1)
	require.NoError(err)
	node2IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node1IPv6)
	err = ping(ctx, node1, inetV6, node1IPv6)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node2IPv6)
	err = ping(ctx, node2, inetV6, node2IPv6)
	require.NoError(err)

	helper.Log("killing nexodus and re-joining nodes with new keys")
	//kill the nexodus process on both nodes
	_, err = helper.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)

	// delete only the public key on node1
	_, err = helper.containerExec(ctx, node1, []string{"rm", "/etc/wireguard/public.key"})
	require.NoError(err)
	// delete the entire wireguard directory on node2
	_, err = helper.containerExec(ctx, node2, []string{"rm", "-rf", "/etc/wireguard/"})
	require.NoError(err)

	// start nexodus on the nodes
	go helper.runNexd(ctx, node1, "--username", username, "--password", password)
	go helper.runNexd(ctx, node2, "--username", username, "--password", password)

	var newNode1IP string
	var newNode1IPv6 string
	err = backoff.Retry(
		func() error {
			var err error
			newNode1IP, err = getContainerIfaceIP(ctx, inetV4, "wg0", node1)
			if err != nil {
				return err
			}
			if newNode1IP == node1IP {
				return fmt.Errorf("new node1IP is the same as the old ip, it should be the next addr in the pool")
			}
			newNode1IPv6, err = getContainerIfaceIP(ctx, inetV6, "wg0", node1)
			if err != nil {
				return err
			}
			if newNode1IPv6 == node1IPv6 {
				return fmt.Errorf("new node1IPv6 is the same as the old ip, it should be the next addr in the pool")
			}
			return nil
		},
		backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx),
	)
	require.NoError(err)

	var newNode2IP string
	var newNode2IPv6 string
	err = backoff.Retry(
		func() error {
			var err error
			newNode2IP, err = getContainerIfaceIP(ctx, inetV4, "wg0", node2)
			if err != nil {
				return err
			}
			if newNode2IP == node2IP {
				return fmt.Errorf("new node1IP is the same as the old ip, it should be the next addr in the pool")
			}
			newNode2IPv6, err = getContainerIfaceIP(ctx, inetV6, "wg0", node2)
			if err != nil {
				return err
			}
			if newNode2IPv6 == node2IPv6 {
				return fmt.Errorf("new node1IPv6 is the same as the old ip, it should be the next addr in the pool")
			}
			return nil
		},
		backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx),
	)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", newNode2IP)
	err = ping(ctx, node1, inetV4, newNode2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", newNode1IP)
	err = ping(ctx, node2, inetV4, newNode1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", newNode2IPv6)
	err = ping(ctx, node1, inetV6, newNode2IPv6)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", newNode1IPv6)
	err = ping(ctx, node2, inetV6, newNode1IPv6)
	require.NoError(err)
}

// TestRequestIPOrganization tests requesting a specific address in a newly created organization
func TestRequestIPOrganization(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()
	node2IP := "100.100.0.102"

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--discovery-node", "--relay-node",
		"--username", username, "--password", password)

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2,
		"--username", username, "--password", password,
		fmt.Sprintf("--request-ip=%s", node2IP),
	)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	// ping the requested IP address (--request-ip)
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Log("killing nexodus and re-joining nodes")
	//kill the nexodus process on both nodes
	_, err = helper.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)

	// restart nexodus and ensure the nodes receive the same re-quested address
	helper.Log("Restarting nexodus on two spoke nodes and re-joining")
	helper.runNexd(ctx, node1, "--discovery-node", "--relay-node",
		"--username", username, "--password", password,
		fmt.Sprintf("--request-ip=%s", node1IP),
	)

	// validate nexd has started on the discovery node
	err = helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2,
		"--username", username, "--password", password,
		fmt.Sprintf("--request-ip=%s", node2IP),
	)

	// ping the requested IP address (--request-ip)
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)
}

// TestHubOrganization test a hub organization with 3 nodes, the first being a relay node
func TestHubOrganization(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()
	node3, stop := helper.CreateNode(ctx, "node3", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--discovery-node", "--relay-node", "--username", username, "--password", password)

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password)
	helper.runNexd(ctx, node3, "--username", username, "--password", password)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)
	node3IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node3)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node3IP)
	err = ping(ctx, node1, inetV4, node3IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node1IP)
	err = ping(ctx, node2, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, inetV4, node3IP)
	require.NoError(err)

	hubOrganizationChildPrefix := "10.188.100.0/24"
	node2ChildPrefixLoopbackNet := "10.188.100.1/32"

	t.Logf("killing nexodus on node2")

	_, err = helper.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)
	t.Logf("rejoining on node2 with --child-prefix=%s", hubOrganizationChildPrefix)

	// add a loopback that are contained in the node's child prefix
	_, err = helper.containerExec(ctx, node2, []string{"ip", "addr", "add", node2ChildPrefixLoopbackNet, "dev", "lo"})
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password,
		fmt.Sprintf("--child-prefix=%s", hubOrganizationChildPrefix),
	)

	// validate nexd has started on the discovery node
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	// address will be the same, this is just a readiness check for gather data
	node1IP, err = getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err = getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)
	node3IP, err = getContainerIfaceIP(ctx, inetV4, "wg0", node3)
	require.NoError(err)

	// parse the loopback ip from the loopback prefix
	node2LoopbackIP, _, _ := net.ParseCIDR(node2ChildPrefixLoopbackNet)

	t.Logf("Pinging loopback on node2 %s from node3 wg0", node2LoopbackIP.String())
	err = ping(ctx, node2, inetV4, node2LoopbackIP.String())
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node3IP)
	err = ping(ctx, node1, inetV4, node3IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node1IP)
	err = ping(ctx, node2, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, inetV4, node3IP)
	require.NoError(err)

	// get the device id for node3
	commandOut, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json",
		"user", "get-current",
	)
	require.NoErrorf(err, "nexctl user get-current error: %v\n", err)
	var user models.UserJSON
	err = json.Unmarshal([]byte(commandOut), &user)
	require.NoErrorf(err, "nexctl user get-current error: %v\n", err)

	commandOut, err = helper.runCommand(nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"organization", "list",
	)
	require.NoErrorf(err, "nexctl user list error: %v\n", err)
	var organizations []models.OrganizationJSON
	err = json.Unmarshal([]byte(commandOut), &organizations)
	require.NoErrorf(err, "nexctl user Unmarshal error: %v\n", err)

	require.Equal(1, len(organizations))
	orgID := organizations[0].ID

	allDevices, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json",
		"device", "list", "--organization-id", orgID.String(),
	)
	require.NoErrorf(err, "nexctl device list error: %v\n", err)
	var devices []models.Device
	err = json.Unmarshal([]byte(allDevices), &devices)
	require.NoErrorf(err, "nexctl device Unmarshal error: %v\n", err)

	// register node3 device ID for node3 for deletion
	var device3ID string
	node3Hostname, err := helper.getNodeHostname(ctx, node3)
	helper.Logf("deleting node3 running in container: %s", node3Hostname)
	require.NoError(err)
	for _, p := range devices {
		if p.Hostname == node3Hostname {
			node3IP = p.TunnelIP
			device3ID = p.ID.String()
		}
	}

	// delete the device node2
	_, err = helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"device", "delete",
		"--device-id", device3ID,
	)
	require.NoError(err)

	// this is probably more time than needed for convergence as polling is currently 5s
	time.Sleep(time.Second * 10)

	// verify the deleted device details are no longer in a device's tables
	node2routes := helper.routesDumpV4(ctx, node2)
	node2dump := helper.wgDump(ctx, node2)

	require.NotContainsf(node2routes, node3IP, "found deleted device node still in routing tables of a device")
	require.NotContainsf(node2dump, node3IP, "found deleted device node still in wg show wg0 dump tables of a device")
}

// TestChildPrefix tests requesting a specific address in a newly created organization
func TestChildPrefix(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()
	node3LoopbackNet := "172.16.10.101/32"
	node2LoopbackNet := "172.16.20.102/32"
	node3ChildPrefix := "172.16.10.0/24"
	node2ChildPrefix := "172.16.20.0/24"

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()
	node3, stop := helper.CreateNode(ctx, "node3", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--discovery-node", "--relay-node",
		"--username", username, "--password", password,
	)

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2,
		fmt.Sprintf("--child-prefix=%s", node2ChildPrefix),
		"--username", username, "--password", password,
	)

	helper.runNexd(ctx, node3,
		fmt.Sprintf("--child-prefix=%s", node3ChildPrefix),
		"--username", username, "--password", password,
	)

	// add loopbacks to the containers that are contained in the node's child prefix
	_, err = helper.containerExec(ctx, node3, []string{"ip", "addr", "add", node3LoopbackNet, "dev", "lo"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"ip", "addr", "add", node2LoopbackNet, "dev", "lo"})
	require.NoError(err)

	// parse the loopback ip from the loopback prefix
	node3LoopbackIP, _, _ := net.ParseCIDR(node3LoopbackNet)
	node2LoopbackIP, _, _ := net.ParseCIDR(node2LoopbackNet)

	// address will be the same, this is just a readiness check for gather data
	node3IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node3)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node2IP)
	err = ping(ctx, node3, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, inetV4, node3IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node2LoopbackIP)
	err = ping(ctx, node3, inetV4, node2LoopbackIP.String())
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3LoopbackIP)
	err = ping(ctx, node2, inetV4, node3LoopbackIP.String())
	require.NoError(err)
}

// TestRelay validates the scenario where the agent is set to explicitly relay only.
func TestRelay(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()
	node3, stop := helper.CreateNode(ctx, "node3", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password)
	helper.runNexd(ctx, node3, "--username", username, "--password", password, "--relay-only")

	// v4 relay connectivity checks
	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)
	node3IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node3IP)
	err = ping(ctx, node1, inetV4, node3IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3IP)
	err = ping(ctx, node2, inetV4, node3IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node2IP)
	err = ping(ctx, node2, inetV4, node2IP)
	require.NoError(err)

	// v6 relay connectivity checks
	node1IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node1)
	require.NoError(err)
	node2IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node2)
	require.NoError(err)
	node3IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node3IPv6)
	err = ping(ctx, node1, inetV6, node3IPv6)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3IPv6)
	err = ping(ctx, node2, inetV6, node3IPv6)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node1IPv6)
	err = ping(ctx, node2, inetV6, node1IPv6)
	require.NoError(err)

	helper.Logf("Pinging %s from node3", node2IPv6)
	err = ping(ctx, node2, inetV6, node2IPv6)
	require.NoError(err)
}

func TestNexctl(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// validate nexctl user get-current returns a user
	commandOut, err := helper.runCommand(nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"user", "get-current",
	)
	require.NoErrorf(err, "nexctl user list error: %v\n", err)
	var user models.UserJSON
	err = json.Unmarshal([]byte(commandOut), &user)
	require.NoErrorf(err, "nexctl user Unmarshal error: %v\n", err)

	require.NotEmpty(user)
	require.NotEmpty(user.ID)
	require.NotEmpty(user.UserName)

	commandOut, err = helper.runCommand(nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"organization", "list",
	)
	require.NoErrorf(err, "nexctl user list error: %v\n", err)
	var organizations []models.OrganizationJSON
	err = json.Unmarshal([]byte(commandOut), &organizations)
	require.NoErrorf(err, "nexctl user Unmarshal error: %v\n", err)
	require.Equal(1, len(organizations))

	// validate no org fields are empty
	require.NotEmpty(organizations[0].ID)
	require.NotEmpty(organizations[0].Name)
	require.NotEmpty(organizations[0].IpCidr)
	require.NotEmpty(organizations[0].IpCidrV6)
	require.NotEmpty(organizations[0].Description)

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--discovery-node", "--relay-node", "--username", username, "--password", password)

	// validate nexd has started on the discovery node
	err = helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "--child-prefix=100.22.100.0/24")

	// validate nexd has started
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	node1IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node1)
	require.NoError(err)
	node2IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node2)
	require.NoError(err)

	// validate list devices and register IDs and IPs
	allDevices, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json-raw",
		"device", "list",
	)
	require.NoErrorf(err, "nexctl device list error: %v\n", err)
	var devices []models.Device
	err = json.Unmarshal([]byte(allDevices), &devices)
	require.NoErrorf(err, "nexctl device Unmarshal error: %v\n", err)

	// validate device fields that should always have values
	require.NotEmpty(devices[0].ID)
	require.NotEmpty(devices[0].Endpoints)
	require.NotEmpty(devices[0].Hostname)
	require.NotEmpty(devices[0].PublicKey)
	require.NotEmpty(devices[0].TunnelIP)
	require.NotEmpty(devices[0].TunnelIpV6)
	require.NotEmpty(devices[0].AllowedIPs)
	require.NotEmpty(devices[0].OrganizationID)
	require.NotEmpty(devices[0].OrganizationPrefix)
	require.NotEmpty(devices[0].OrganizationPrefixV6)
	require.NotEmpty(devices[0].EndpointLocalAddressIPv4)
	// TODO: add assert.NotEmpty(devices[0].ReflexiveIPv4) with #739

	// register the device IDs for node1 and node2 for deletion
	var node1DeviceID string
	var node2DeviceID string
	node1Hostname, err := helper.getNodeHostname(ctx, node1)
	helper.Logf("deleting Node1 running in container: %s", node1Hostname)
	require.NoError(err)
	node2Hostname, err := helper.getNodeHostname(ctx, node2)
	helper.Logf("deleting Node2 running in container: %s", node2Hostname)
	require.NoError(err)

	for _, p := range devices {
		if p.Hostname == node1Hostname {
			node1DeviceID = p.ID.String()
		}
		if p.Hostname == node2Hostname {
			node2DeviceID = p.ID.String()
		}
	}

	//kill the nexodus process on both nodes
	_, err = helper.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)

	// delete both devices from nexodus
	node1Delete, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"device", "delete",
		"--device-id", node1DeviceID,
	)
	require.NoError(err)
	helper.Logf("nexctl node1 delete results: %s", node1Delete)
	node2Delete, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"device", "delete",
		"--device-id", node2DeviceID,
	)
	require.NoError(err)
	helper.Logf("nexctl node2 delete results: %s", node2Delete)

	// delete the keys on both nodes to force ensure the deleted device released it's
	// IPAM address and will re-issue that address to a new device with a new keypair.
	_, err = helper.containerExec(ctx, node1, []string{"rm", "-rf", "/etc/wireguard/"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"rm", "-rf", "/etc/wireguard/"})
	require.NoError(err)

	time.Sleep(time.Second * 10)
	// re-join both nodes, flipping the child-prefix to node1 to ensure the child-prefix was released
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--child-prefix=100.22.100.0/24")

	// validate nexd has started on the discovery node
	err = helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password)

	// validate nexd has started
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	newNode1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	// If the device was not deleted, the next registered device would receive the
	// next available address in the IPAM pool, not the previously assigned address.
	// Fail the test if the device IP was not the previous address from the IPAM pool.
	var addressMatch bool
	if newNode1IP == node2IP {
		addressMatch = true
		helper.Logf("Pinging %s from node1", node1IP)
		err = ping(ctx, node1, inetV4, node1IP)
		require.NoError(err)
	}
	if newNode1IP == node1IP {
		addressMatch = true
		helper.Logf("Pinging %s from node1", node2IP)
		err = ping(ctx, node1, inetV4, node2IP)
		require.NoError(err)
	}
	if !addressMatch {
		require.Failf("ipam/device IPv4 delete failed", fmt.Sprintf("Node did not receive the proper IPAM IPv4 address %s, it should have been %s or %s", newNode1IP, node1IP, node2IP))
	}

	// same as above but for v6, ensure IPAM released the leases from the deleted nodes and re-issued them
	newNode1IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node1)
	require.NoError(err)

	var addressMatchV6 bool
	if newNode1IPv6 == node2IPv6 {
		addressMatchV6 = true
		helper.Logf("Pinging %s from node1", node1IPv6)
		err = ping(ctx, node1, inetV6, node1IPv6)
		require.NoError(err)
	}
	if newNode1IPv6 == node1IPv6 {
		addressMatchV6 = true
		helper.Logf("Pinging %s from node1", node2IPv6)
		err = ping(ctx, node1, inetV6, node2IPv6)
		require.NoError(err)
	}
	if !addressMatchV6 {
		require.Failf("ipam/device IPv6 delete failed", fmt.Sprintf("Node did not receive the proper IPAM IPv6 address %s, it should have been %s or %s", newNode1IPv6, node1IPv6, node2IPv6))
	}

	// validate list devices in a organization
	devicesInOrganization, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json-raw",
		"device", "list",
		"--organization-id", organizations[0].ID.String(),
	)
	require.NoErrorf(err, "nexctl device list error: %v\n", err)

	err = json.Unmarshal([]byte(devicesInOrganization), &devices)
	require.NoErrorf(err, "nexctl device Unmarshal error: %v\n", err)

	// List users and register the current user's ID for deletion
	userList, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json-raw",
		"user", "list",
	)
	require.NoErrorf(err, "nexctl user list error: %v\n", err)
	var users []models.User
	err = json.Unmarshal([]byte(userList), &users)
	require.NoErrorf(err, "nexctl user Unmarshal error: %v\n", err)

	var deleteUserID string
	for _, u := range users {
		if u.UserName == username {
			deleteUserID = u.ID
		}
	}
	_, err = helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"user", "delete",
		"--user-id", deleteUserID,
	)
	require.NoError(err)

	// users get auto recreated... for this to work another user would need to do the check
	// negative test ensuring the user was deleted
	//_, err = helper.runCommand(nexctl,
	//	"--username", username,
	//	"--password", password,
	//	"user", "list",
	//)
	//require.Error(err)
}

// TestV6Disabled validate that a node that does support ipv6 provisions with v4 successfully
func TestV6Disabled(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()
	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, disableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, disableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password)
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	// TODO: add v6 disabled support to gather
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)
}

// TestProxyEgress tests that nexd proxy can be used with a single egress rule
func TestProxyEgress(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy", "--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node1
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node1, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node1, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node2 (to the local proxy) to reach the server on node1
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	wg.Wait()
}

// TestProxyEgress tests that nexd proxy can be used with multiple egress rules
func TestProxyEgressMultipleRules(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")),
		"--egress", fmt.Sprintf("tcp:81:%s", net.JoinHostPort(node1IP, "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node1
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node1, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node1, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node2 (to the local proxy) to reach the server on node1
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		output2, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost:81"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output2)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		require.True(strings.Contains(output2, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	wg.Wait()
}

// TestProxyIngress tests that nexd proxy with a single ingress rule
func TestProxyIngress(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy", "--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node2
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node2, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node2, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node1 to the server on node2 (running the proxy)
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(node2IP, "8080"))})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	wg.Wait()
}

// TestProxyIngress tests that nexd proxy with multiple ingress rules
func TestProxyIngressMultipleRules(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")),
		"--ingress", fmt.Sprintf("tcp:8081:%s", net.JoinHostPort("127.0.0.1", "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node2
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node2, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node2, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node1 to the server on node2 (running the proxy)
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(node2IP, "8080"))})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		output2, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(node2IP, "8081"))})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output2)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	wg.Wait()
}

// TestProxyIngressAndEgress tests that a proxy can be used to both ingress and egress traffic
func TestProxyIngressAndEgress(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--discovery-node", "--relay-node")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")),
		"--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node1 and node2
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node2, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node2, []string{"python3", "-m", "http.server", "8080"})
	})
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node1, []string{"python3", "-c", "import os; open('index.html', 'w').write('pancakes')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node1, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node1 to the server on node2 (this exercises the egress rule)
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(node2IP, "8080"))})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)

	// run curl on node2 (to the local proxy) to reach the server on node1 (this exercises the ingress rule)
	ctxTimeout, curlCancel = context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err = util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "pancakes"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	wg.Wait()
}

func TestApiClientConflictError(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	c, err := client.NewAPIClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
		username,
		password,
	))
	require.NoError(err)
	user, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
	require.NoError(err)
	orgs, _, err := c.OrganizationsApi.ListOrganizations(ctx).Execute()
	require.NoError(err)

	privateKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(err)
	publicKey := privateKey.PublicKey().String()

	device, _, err := c.DevicesApi.CreateDevice(ctx).Device(public.ModelsAddDevice{
		EndpointLocalAddressIp4: "172.17.0.3",
		Hostname:                "bbac3081d5e8",
		OrganizationId:          orgs[0].Id,
		PublicKey:               publicKey,
		UserId:                  user.Id,
		Endpoints: []public.ModelsEndpoint{
			{
				Source:   "local",
				Address:  "172.17.0.3:58664",
				Distance: 0,
			},
			{
				Source:   "stun:",
				Address:  "47.196.141.165",
				Distance: 12,
			},
		},
	}).Execute()
	require.NoError(err)

	_, resp, err := c.DevicesApi.CreateDevice(ctx).Device(public.ModelsAddDevice{
		EndpointLocalAddressIp4: "172.17.0.3",
		Hostname:                "bbac3081d5e8",
		OrganizationId:          orgs[0].Id,
		PublicKey:               publicKey,
		UserId:                  user.Id,
		Endpoints: []public.ModelsEndpoint{
			{
				Source:   "local",
				Address:  "172.17.0.3:58664",
				Distance: 0,
			},
			{
				Source:   "stun:",
				Address:  "47.196.141.165",
				Distance: 12,
			},
		},
	}).Execute()
	require.Error(err)
	require.NotNil(resp)

	var apiError *public.GenericOpenAPIError
	require.True(errors.As(err, &apiError))

	conflict, ok := apiError.Model().(public.ModelsConflictsError)
	require.True(ok)
	require.Equal(device.Id, conflict.Id)
}
func TestWatchDevices(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	assert := helper.assert
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cancel := helper.createNewUser(ctx, password)
	defer cancel()

	c, err := client.NewAPIClient(ctx, "https://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(
		username,
		password,
	))
	require.NoError(err)
	user, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
	require.NoError(err)
	orgs, _, err := c.OrganizationsApi.ListOrganizations(ctx).Execute()
	require.NoError(err)

	privateKey, err := wgtypes.GeneratePrivateKey()
	require.NoError(err)
	publicKey := privateKey.PublicKey().String()

	watch, _, err := c.DevicesApi.ListDevicesInOrganization(ctx, orgs[0].Id).Watch()
	require.NoError(err)
	defer watch.Close()

	kind, _, err := watch.Receive()
	require.NoError(err)
	assert.Equal("bookmark", kind)

	device, _, err := c.DevicesApi.CreateDevice(ctx).Device(public.ModelsAddDevice{
		EndpointLocalAddressIp4: "172.17.0.3",
		Hostname:                "bbac3081d5e8",
		OrganizationId:          orgs[0].Id,
		PublicKey:               publicKey,
		UserId:                  user.Id,
		Endpoints: []public.ModelsEndpoint{
			{
				Source:   "local",
				Address:  "172.17.0.3:58664",
				Distance: 0,
			},
			{
				Source:   "stun:",
				Address:  "47.196.141.165",
				Distance: 12,
			},
		},
	}).Execute()
	require.NoError(err)

	// We should get sent an event for the device that was created
	kind, watchedDevice, err := watch.Receive()
	require.NoError(err)
	assert.Equal("change", kind)
	assert.Equal(*device, watchedDevice)

	device, _, err = c.DevicesApi.DeleteDevice(ctx, device.Id).Execute()
	require.NoError(err)

	// We should get sent an event for the device that was deleted
	kind, watchedDevice, err = watch.Receive()
	require.NoError(err)
	assert.Equal("delete", kind)
	assert.Equal(*device, watchedDevice)

}
