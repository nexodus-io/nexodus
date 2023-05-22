//go:build integration

package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/testcontainers/testcontainers-go"
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")

	// validate nexd has started on the relay node
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")

	// validate nexd has started on the relay node
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password,
		fmt.Sprintf("--request-ip=%s", node1IP), "relay")

	// validate nexd has started on the relay node
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

// TestChooseOrganization tests choosing an organization when creating a new node
func TestChooseOrganization(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	orgs := []struct {
		id     string
		cidr   string
		cidrV6 string
	}{
		{
			cidr:   "100.110.0.0/16",
			cidrV6: "210::/64",
		},
		{
			cidr:   "100.111.0.0/16",
			cidrV6: "211::/64",
		},
	}

	for i, org := range orgs {
		orgOut, err := helper.runCommand(nexctl,
			"--username", username, "--password", password,
			"--output", "json",
			"organization", "create",
			"--name", fmt.Sprintf("%s-%s-org%d", "TestChooseOrganization", time.Now().Format("2006-01-02-15-04-05"), i),
			"--description", "Test Org",
			"--cidr", org.cidr,
			"--cidr-v6", org.cidrV6,
		)
		require.NoError(err)
		helper.Logf("Output from creating org: %s", orgOut)
		var org models.OrganizationJSON
		err = json.Unmarshal([]byte(orgOut), &org)
		require.NoErrorf(err, "nexctl organization Unmarshal error: %v\n", err)
		orgs[i].id = org.ID.String()

		defer func(orgID string) {
			_, _ = helper.runCommand(nexctl,
				nexctl,
				"--username", username, "--password", password,
				"organization", "delete", "--organization-id", orgID)
		}(orgs[i].id)
	}

	useOrgs := []string{
		"",         // default org
		orgs[0].id, // change to a custom org
		orgs[1].id, // change to another customer org
		orgs[0].id, // change back to a previous org
	}

	// Re-use the same 2 nodes for each test case. We want to keep the
	// same keys so we're moving the same device around between orgs.
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	lastIPs := map[string]string{
		"node1IP":   "",
		"node2IP":   "",
		"node1IPv6": "",
		"node2IPv6": "",
	}
	for _, orgID := range useOrgs {
		args := []string{"--username", username, "--password", password}
		if orgID != "" {
			args = append(args, "--org-id", orgID)
		}

		// start nexd on node1
		helper.runNexd(ctx, node1, args...)
		err := helper.nexdStatus(ctx, node1)
		require.NoError(err)

		// start nexd on node2
		helper.runNexd(ctx, node2, args...)
		err = helper.nexdStatus(ctx, node2)
		require.NoError(err)

		// get tunnel IPs for node1, validate that they changed from the last org used
		node1IP, err := getTunnelIP(ctx, helper, inetV4, node1)
		require.NoError(err)
		require.NotEqual(lastIPs["node1IP"], node1IP)
		lastIPs["node1IP"] = node1IP
		node1IPv6, err := getTunnelIP(ctx, helper, inetV6, node1)
		require.NoError(err)
		require.NotEqual(lastIPs["node1IPv6"], node1IPv6)
		lastIPs["node1IPv6"] = node1IPv6

		// get tunnel IPs for node2, validate that they changed from the last org used
		node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
		require.NoError(err)
		require.NotEqual(lastIPs["node2IP"], node2IP)
		lastIPs["node2IP"] = node2IP
		node2IPv6, err := getTunnelIP(ctx, helper, inetV6, node2)
		require.NoError(err)
		require.NotEqual(lastIPs["node2IPv6"], node2IPv6)
		lastIPs["node2IPv6"] = node2IPv6

		// ping node2 from node1
		helper.Logf("Pinging %s from node1", node2IP)
		err = ping(ctx, node1, inetV4, node2IP)
		require.NoError(err)
		helper.Logf("Pinging %s from node1", node2IPv6)
		err = ping(ctx, node1, inetV6, node2IPv6)
		require.NoError(err)

		// ping node1 from node2
		helper.Logf("Pinging %s from node2", node1IP)
		err = ping(ctx, node2, inetV4, node1IP)
		require.NoError(err)
		helper.Logf("Pinging %s from node2", node1IPv6)
		err = ping(ctx, node2, inetV6, node1IPv6)
		require.NoError(err)

		// kill nexd on both nodes
		_, err = helper.containerExec(ctx, node1, []string{"killall", "nexd"})
		require.NoError(err)
		_, err = helper.containerExec(ctx, node2, []string{"killall", "nexd"})
		require.NoError(err)
		err = helper.nexdStopped(ctx, node1)
		require.NoError(err)
		err = helper.nexdStopped(ctx, node2)
		require.NoError(err)
	}
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")

	// validate nexd has started on the relay node
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
		"router", fmt.Sprintf("--child-prefix=%s", hubOrganizationChildPrefix),
	)

	// validate nexd has started on the relay node
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

	require.NotContainsf(node2routes, node3IP, "found deleted device node still in routing tables of a device")
}

// TestChildPrefix tests requesting a specific address in a newly created organization for v4 and v6. This will start nexd three
// different times. The first makes sure the prefix is created and routes are added. The second is started and then killed.
// The third start of nexd is to validate the child-prefix was not deleted from the ipam database. TODO: test changing the child-prefix
func TestChildPrefix(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()
	node2LoopbackNet := "172.16.20.102/32"
	node3LoopbackNet := "172.16.10.101/32"
	node2LoopbackNetV6 := "200:2::1/64"
	node3LoopbackNetV6 := "200:3::1/64"
	node2ChildPrefix := "172.16.20.0/24,200:2::/64"
	node3ChildPrefix := "172.16.10.0/24,200:3::/64"

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()
	node3, stop := helper.CreateNode(ctx, "node3", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1,
		"--username", username, "--password", password,
		"relay",
	)

	// validate nexd has started on the relay node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2,
		"--username", username, "--password", password,
		"router", fmt.Sprintf("--child-prefix=%s", node2ChildPrefix),
	)

	helper.runNexd(ctx, node3,
		"--username", username, "--password", password,
		"router", fmt.Sprintf("--child-prefix=%s", node3ChildPrefix),
	)

	// add v4 loopbacks to the containers that are contained in the node's child prefix
	_, err = helper.containerExec(ctx, node3, []string{"ip", "addr", "add", node3LoopbackNet, "dev", "lo"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"ip", "addr", "add", node2LoopbackNet, "dev", "lo"})
	require.NoError(err)
	// add v6 loopbacks to the containers that are contained in the node's child prefix
	_, err = helper.containerExec(ctx, node3, []string{"ip", "-6", "addr", "add", node3LoopbackNetV6, "dev", "lo"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"ip", "-6", "addr", "add", node2LoopbackNetV6, "dev", "lo"})
	require.NoError(err)

	// parse the loopback ip from the loopback prefix
	node3LoopbackIP, _, _ := net.ParseCIDR(node3LoopbackNet)
	node2LoopbackIP, _, _ := net.ParseCIDR(node2LoopbackNet)
	// parse the loopback ipv6 from the loopback prefix
	node3LoopbackIPv6, _, _ := net.ParseCIDR(node3LoopbackNetV6)
	node2LoopbackIPv6, _, _ := net.ParseCIDR(node2LoopbackNetV6)

	// readiness check
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)
	err = helper.nexdStatus(ctx, node3)
	require.NoError(err)

	// gather the wg0 v4 addresses
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

	helper.Logf("Pinging %s from node3", node2LoopbackIPv6)
	err = ping(ctx, node3, inetV6, node2LoopbackIPv6.String())
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3LoopbackIPv6)
	err = ping(ctx, node2, inetV6, node3LoopbackIPv6.String())
	require.NoError(err)

	// kill the nexodus process on all nodes
	_, err = helper.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node2, []string{"killall", "nexd"})
	require.NoError(err)
	_, err = helper.containerExec(ctx, node3, []string{"killall", "nexd"})
	require.NoError(err)

	// start nexd two more times, only validate connectivity on the second.
	for i := 0; i < 2; i++ {
		// start nexodus on the nodes
		helper.runNexd(ctx, node1,
			"--username", username, "--password", password,
			"relay",
		)

		// validate nexd has started on the relay node
		err := helper.nexdStatus(ctx, node1)
		require.NoError(err)

		helper.runNexd(ctx, node2,
			"--username", username, "--password", password,
			"router", fmt.Sprintf("--child-prefix=%s", node2ChildPrefix),
		)

		helper.runNexd(ctx, node3,
			"--username", username, "--password", password,
			"router", fmt.Sprintf("--child-prefix=%s", node3ChildPrefix),
		)

		// readiness check
		err = helper.nexdStatus(ctx, node2)
		require.NoError(err)
		err = helper.nexdStatus(ctx, node3)
		require.NoError(err)
		// kill nexd only on the 1st run in the loop
		if i == 0 {
			//kill the nexodus process on all three nodes
			_, err = helper.containerExec(ctx, node1, []string{"killall", "nexd"})
			require.NoError(err)
			_, err = helper.containerExec(ctx, node2, []string{"killall", "nexd"})
			require.NoError(err)
			_, err = helper.containerExec(ctx, node3, []string{"killall", "nexd"})
			require.NoError(err)
		}
	}

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

	helper.Logf("Pinging %s from node3", node2LoopbackIPv6)
	err = ping(ctx, node3, inetV6, node2LoopbackIPv6.String())
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node3LoopbackIPv6)
	err = ping(ctx, node2, inetV6, node3LoopbackIPv6.String())
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")

	// validate nexd has started on the relay node
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")

	// validate nexd has started on the relay node
	err = helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "router", "--child-prefix=100.22.100.0/24")

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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "router", "--child-prefix=100.22.100.0/24")

	// validate nexd has started on the relay node
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
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

/*
TestSecurityGroups validate that Security Groups.
All tests will use netcat to open a listening port that will echo the hostname of the
device to anything that connects to the listening port. To test that device, a connection
is attempted to the listening device and verifies it received the hostname of the listener.
If the return results contain the hostname of the listener, the connection was successful.
If the return results are empty, then the connection was unsuccessful.
Since there is a period of time between the request of the policy from the nexctl api
call and the execution of the policy, a retry helper will validate the nftable rules have
changed before the tests are performed. Tests will verify combinations of UDP, TCP, IPv4
and IPv6.

Tests performed are as follows:
1. The security group IDs for the nodes are verified present and org ID is gathered.
2. This section will validate the default policy is allowing port traffic to ports.
3. The second set will add a policy and verify the explicitly allowed ports are reachable.
5. The next section will perform negative tests that will add a new policy and a netcat
listener will be put in place on a port that is not in the policy allow and verify that
// connection is not permitted.
*/
func TestSecurityGroups(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password)
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)
	err = helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password)
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	// validate nexctl user get-current returns a user
	commandOut, err := helper.runCommand(nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"user", "get-current",
	)
	require.NoErrorf(err, "nexctl security-groups list error: %v\n", err)
	var user models.UserJSON
	err = json.Unmarshal([]byte(commandOut), &user)
	require.NoErrorf(err, "nexctl Security Groups unmarshal error: %v\n", err)

	require.NotEmpty(user)
	require.NotEmpty(user.ID)
	require.NotEmpty(user.UserName)

	var node1DeviceID, node2DeviceID, secGroupID, orgID string
	var idExists, node1IdExists, node2IdExists bool

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

	node1Hostname, err := helper.getNodeHostname(ctx, node1)
	helper.Logf("deleting Node1 running in container: %s", node1Hostname)
	require.NoError(err)
	node2Hostname, err := helper.getNodeHostname(ctx, node2)
	helper.Logf("deleting Node2 running in container: %s", node2Hostname)
	require.NoError(err)

	for _, device := range devices {
		if device.Hostname == node1Hostname {
			node1DeviceID = device.ID.String()
			secGroupID = device.SecurityGroupIds.String()
			orgID = device.OrganizationID.String()
		}
		if device.Hostname == node2Hostname {
			node2DeviceID = device.ID.String()
		}
	}
	require.NotEmpty(node1DeviceID)
	require.NotEmpty(node2DeviceID)

	// loop back through and verify the security group ID is present on both devices
	for _, device := range devices {
		if device.Hostname == node1Hostname {
			secGroupID = device.SecurityGroupIds.String()
			orgID = device.OrganizationID.String()
			if secGroupID == device.SecurityGroupIds.String() {
				node1IdExists = true
			}
		} else if device.Hostname == node2Hostname {
			if secGroupID == device.SecurityGroupIds.String() {
				node2IdExists = true
			}
		}
	}

	// fail the test if either device does not contain the organization security group
	if !node1IdExists {
		helper.Errorf("An Security Group ID of %s was not found associated to the devices %+v", orgID, devices)
	}
	if !node2IdExists {
		helper.Errorf("An Security Group ID of %s was not found associated to the devices %+v", orgID, devices)
	}

	// validate list devices and register IDs and IPs
	secGroupJSON, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"--output", "json-raw",
		"security-group", "list",
		"--organization-id", orgID,
	)
	require.NoErrorf(err, "nexctl security group list error: %v\n", err)

	var secGroup []models.SecurityGroup
	err = json.Unmarshal([]byte(secGroupJSON), &secGroup)
	require.NoErrorf(err, "nexctl security group unmarshal error: %v\n", err)

	for _, sg := range secGroup {
		if orgID == sg.OrganizationId.String() && secGroupID == sg.ID.String() {
			idExists = true
			break
		}
	}
	if !idExists {
		helper.Logf("An Organization ID of %s was not found in the Security Group listing %+v", orgID, secGroup)
	}

	// register the v4 and v6 addresses for both devices
	node1IPv4, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IPv4, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)
	node1IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node1)
	require.NoError(err)
	node2IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node2)
	require.NoError(err)

	// v4 TCP port 80 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv4, protoTCP, "11114")
	require.NoError(err)
	connectResults, err := helper.connectToPort(ctx, node2, node1IPv4, protoTCP, "11114")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v4 UDP port 25 should succeed
	err = helper.startPortListener(ctx, node2, node2IPv4, protoUDP, "11124")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node1, node2IPv4, protoUDP, "11124")
	require.NoError(err)
	require.Equal(node2Hostname, connectResults)

	// v6 TCP port 22 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoTCP, "11116")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoTCP, "11116")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v6 UDP port 4567 should succeed
	err = helper.startPortListener(ctx, node2, node2IPv6, protoUDP, "11126")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node1, node2IPv6, protoUDP, "11126")
	require.NoError(err)
	require.Equal(node2Hostname, connectResults)

	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err := helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)

	// Update the security group with the new inbound and outbound rules
	inboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "18", "25", []string{"100.100.0.0/16"}),
		helper.createSecurityRule("tcp", "18", "25", []string{"200::/64"}),
	}
	outboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "0", "0", []string{""}),
	}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	allSucceeded, err := helper.retryCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v4 TCP port 25 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv4, protoTCP, "25")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv4, protoTCP, "25")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v6 TCP port 20 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoTCP, "20")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoTCP, "20")
	require.NoError(err)
	require.Equal(connectResults, node1Hostname)

	// v4 UDP port 24 should fail
	err = helper.startPortListener(ctx, node2, node2IPv4, protoUDP, "24")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node1, node2IPv4, protoUDP, "24")
	require.Empty(connectResults)

	// v6 UDP port 21 should fail
	err = helper.startPortListener(ctx, node2, node2IPv6, protoUDP, "21")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node1, node2IPv6, protoUDP, "21")
	require.Empty(connectResults)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	nfOutBefore, err = helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)

	// create the new inbound and outbound rules
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "0", "0", []string{"200::/64"}),
	}
	outboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv6", "0", "0", []string{""}),
	}

	// update the security group with the new inbound and outbound rules
	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	allSucceeded, err = helper.retryCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v6 tcp 1111 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoTCP, "1111")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoTCP, "1111")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v4 tcp 1111 should fail
	err = helper.startPortListener(ctx, node1, node1IPv4, protoTCP, "1122")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node2, node1IPv4, protoTCP, "1122")
	require.Empty(connectResults)

	// v4 ping should fail
	helper.Logf("Pinging %s from node1", node2IPv4)
	err = pingWithoutRetry(ctx, node1, inetV4, node2IPv4)
	require.Error(err)
	// v6 ping should fail
	helper.Logf("Pinging %s from node1", node2IPv6)
	err = pingWithoutRetry(ctx, node1, inetV6, node2IPv6)
	require.Error(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	nfOutBefore, err = helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)

	// create the new inbound and outbound rules
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("icmpv4", "0", "0", []string{"100.100.0.1-100.100.0.50"}),
		helper.createSecurityRule("icmpv6", "0", "0", []string{""}),
	}
	outboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("icmpv4", "0", "0", []string{""}),
		helper.createSecurityRule("icmpv6", "0", "0", []string{"200::1-200::50"}),
	}

	// update the security group with the new inbound and outbound rules
	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	allSucceeded, err = helper.retryCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v4 ping should succeed
	helper.Logf("Pinging %s from node1", node2IPv4)
	err = pingWithoutRetry(ctx, node1, inetV4, node2IPv4)
	require.NoError(err)
	// v6 ping should succeed
	helper.Logf("Pinging %s from node1", node2IPv6)
	err = pingWithoutRetry(ctx, node1, inetV6, node2IPv6)
	require.NoError(err)
	// v4 ping should succeed
	helper.Logf("Pinging %s from node2", node1IPv4)
	err = pingWithoutRetry(ctx, node2, inetV4, node1IPv4)
	require.NoError(err)
	// v6 ping should succeed
	helper.Logf("Pinging %s from node2", node1IPv6)
	err = pingWithoutRetry(ctx, node2, inetV6, node1IPv6)
	require.NoError(err)
}

// TestSecurityGroupsExtended is a continuation of TestSecurityGroups() tests in order
// to continue testing the various combinations in parallel to reduce e2e run time
func TestSecurityGroupsExtended(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password)
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)
	err = helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password)
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	// validate nexctl user get-current returns a user
	commandOut, err := helper.runCommand(nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"user", "get-current",
	)
	require.NoErrorf(err, "nexctl security-groups list error: %v\n", err)
	var user models.UserJSON
	err = json.Unmarshal([]byte(commandOut), &user)
	require.NoErrorf(err, "nexctl Security Groups unmarshal error: %v\n", err)

	require.NotEmpty(user)
	require.NotEmpty(user.ID)
	require.NotEmpty(user.UserName)

	var node1DeviceID, node2DeviceID, secGroupID, orgID string
	var node1IdExists, node2IdExists bool

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

	node1Hostname, err := helper.getNodeHostname(ctx, node1)
	helper.Logf("deleting Node1 running in container: %s", node1Hostname)
	require.NoError(err)
	node2Hostname, err := helper.getNodeHostname(ctx, node2)
	helper.Logf("deleting Node2 running in container: %s", node2Hostname)
	require.NoError(err)

	for _, device := range devices {
		if device.Hostname == node1Hostname {
			node1DeviceID = device.ID.String()
			secGroupID = device.SecurityGroupIds.String()
			orgID = device.OrganizationID.String()
		}
		if device.Hostname == node2Hostname {
			node2DeviceID = device.ID.String()
		}
	}
	require.NotEmpty(node1DeviceID)
	require.NotEmpty(node2DeviceID)

	// loop back through and verify the security group ID is present on both devices
	for _, device := range devices {
		if device.Hostname == node1Hostname {
			secGroupID = device.SecurityGroupIds.String()
			orgID = device.OrganizationID.String()
			if secGroupID == device.SecurityGroupIds.String() {
				node1IdExists = true
			}
		} else if device.Hostname == node2Hostname {
			if secGroupID == device.SecurityGroupIds.String() {
				node2IdExists = true
			}
		}
	}

	// fail the test if either device does not contain the organization security group
	if !node1IdExists {
		helper.Errorf("An Security Group ID of %s was not found associated to the devices %+v", orgID, devices)
	}
	if !node2IdExists {
		helper.Errorf("An Security Group ID of %s was not found associated to the devices %+v", orgID, devices)
	}

	// register the v4 and v6 addresses for both devices
	node1IPv4, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IPv4, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)
	node1IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node1)
	require.NoError(err)
	node2IPv6, err := getContainerIfaceIP(ctx, inetV6, "wg0", node2)
	require.NoError(err)

	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err := helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)

	// Update the security group with the new inbound and outbound rules
	inboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("udp", "5000", "6000",
			[]string{"100.100.0.0/24",
				"172.28.100.1-172.28.100.100",
				"192.168.168.100",
			}),
		helper.createSecurityRule("udp", "0", "0",
			[]string{"200::/64",
				"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db8:ffff:ffff:ffff:ffff:ffff:ffff",
				"2001:0000:0000:0000:0000:0000:0000:0010",
			}),
	}
	// Verify an empty rule will apply a permit all
	outboundRules := []public.ModelsSecurityRule{}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	allSucceeded, err := helper.retryCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v4 UDP port 5001 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv4, protoUDP, "5001")
	require.NoError(err)
	connectResults, err := helper.connectToPort(ctx, node2, node1IPv4, protoUDP, "5001")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v6 UDP port 5999 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoUDP, "5999")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoUDP, "5999")
	require.NoError(err)
	require.Equal(connectResults, node1Hostname)

	// v4 TCP port 8080 should fail since out of the allowed port range
	err = helper.startPortListener(ctx, node2, node2IPv4, protoUDP, "8080")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node1, node2IPv4, protoUDP, "8080")
	require.Empty(connectResults)

	// v4 TCP port 5050 should fail since TCP is not permitted
	err = helper.startPortListener(ctx, node2, node2IPv4, protoTCP, "5050")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node1, node2IPv4, protoTCP, "5050")
	require.Empty(connectResults)

	// v6 UDP port 5051 should fail since TCP is not permitted
	err = helper.startPortListener(ctx, node2, node2IPv6, protoTCP, "5051")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node1, node2IPv6, protoTCP, "5051")
	require.Empty(connectResults)

	// v4 ping should fail since ICMPv4 is not permitted
	helper.Logf("Pinging %s from node2", node1IPv4)
	err = pingWithoutRetry(ctx, node2, inetV4, node1IPv4)
	require.Error(err)
	// v6 ping should fail since ICMPv6 is not permitted
	helper.Logf("Pinging %s from node2", node1IPv6)
	err = pingWithoutRetry(ctx, node2, inetV6, node1IPv6)
	require.Error(err)

	// manually delete the netfilter table to ensure it is recreated when the next group is applied
	_, err = helper.containerExec(ctx, node2, []string{"nft", "flush", "ruleset"})
	require.NoError(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	nfOutBefore, err = helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)

	// create the new inbound and outbound rules
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "0", "0", []string{}),
		helper.createSecurityRule("icmpv4", "0", "0", []string{}),
	}
	outboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "0", "0", []string{}),
		helper.createSecurityRule("icmpv4", "0", "0", []string{}),
	}

	// update the security group with the new inbound and outbound rules
	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	allSucceeded, err = helper.retryCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v6 tcp 8080 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoTCP, "8080")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoTCP, "8080")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v4 UDP 9000 should fail
	err = helper.startPortListener(ctx, node1, node1IPv4, protoUDP, "9000")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node2, node1IPv4, protoUDP, "9000")
	require.Empty(connectResults)

	// v4 ping should succeed
	helper.Logf("Pinging %s from node1", node2IPv4)
	err = pingWithoutRetry(ctx, node1, inetV4, node2IPv4)
	require.NoError(err)
	// v6 ping should fail
	helper.Logf("Pinging %s from node1", node2IPv6)
	err = pingWithoutRetry(ctx, node1, inetV6, node2IPv6)
	require.Error(err)
}
