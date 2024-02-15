//go:build integration

package integration_tests

import (
	"context"
	"os"
	"testing"
	"time"
)

// Testing various connectivity scenarios that requires DERP relay server.
// These test mimic both public and on-boarded DERP relays.

// TestConnectivityFailureWithoutDerpRelay validates the scenario where the
// peers behind symmetric NATs are not able to connect to each other if derp
// relay (public or on-boarded) is not available.
func TestConnectivityFailureWithoutDerpRelay(t *testing.T) {
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// Make sure that the nodes don't reach to the public derp relay
	// that is running at relay.nexodus.io
	err := os.Setenv("NEX_DERP_RELAY_IP", "127.0.0.1")
	require.NoError(err)

	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// Start the nodes in relay-only mode to mimic the symmetric NAT scenario
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--relay-only")
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "--relay-only")

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	// Ping should fail and return an error
	helper.Logf("Pinging %s from node2", node1IP)
	err = pingWithoutRetry(ctx, node2, inetV4, node1IP)
	require.Error(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = pingWithoutRetry(ctx, node1, inetV4, node2IP)
	require.Error(err)
}

// TestConnectivityViaPublicRelay1 validates the scenario where the
// peers behind symmetric NATs are able to connect to each other through a
// public relay. Public relay is deployed as a part of kind nexodus stack
// with self signed certificates.
func TestConnectivityViaPublicRelay1(t *testing.T) {
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the derp relay node
	relay, stop := helper.CreateNode(ctx, "derp-relay", []string{defaultNetwork}, enableV6)
	defer stop()
	relayIp, err := relay.ContainerIP(ctx)
	require.NoError(err)

	// Set the relay IP as an environment variable so createNode can use it to
	// set the DNS entry pointing to the relay.
	err = os.Setenv("NEX_DERP_RELAY_IP", relayIp)
	require.NoError(err)

	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start derp relay node without onboarding, mimicking the public relay scenario
	helper.runNexd(ctx, relay, "--username", username, "--password", password, "relayderp",
		"--hostname relay.nexodus.io", "--certmode manual", "--certdir /.certs/")

	// Start both the nodes in the relay-only mode to mimic the symmetric NAT scenario
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--relay-only")
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "--relay-only")

	// v4 relay connectivity checks
	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)
}

// TestConnectivityViaPublicRelay2 validates the scenario where one peer is
// behind symmetric NATs and other behind reflexive address, are able to
// connect to each other through a public relay. Public relay is deployed
// as a part of kind nexodus stack with self signed certificates.
func TestConnectivityViaPublicRelay2(t *testing.T) {
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the derp relay node
	relay, stop := helper.CreateNode(ctx, "derp-relay", []string{defaultNetwork}, enableV6)
	defer stop()
	relayIp, err := relay.ContainerIP(ctx)
	require.NoError(err)

	// Set the relay IP as an environment variable so createNode can use it to
	// set the DNS entry pointing to the relay.
	err = os.Setenv("NEX_DERP_RELAY_IP", relayIp)
	require.NoError(err)

	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start derp relay node without onboarding, mimicking the public relay scenario
	helper.runNexd(ctx, relay, "--username", username, "--password", password, "relayderp",
		"--hostname relay.nexodus.io", "--certmode manual", "--certdir /.certs/")

	// Start one node with relay-only to mimic symmetric NAT and other behind reflexive address
	helper.runNexd(ctx, node1, "--username", username, "--password", password)
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "--relay-only")

	// v4 relay connectivity checks
	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)
}

// TestConnectivityViaOnboardedRelay1 validates the scenario where the
// peers behind symmetric NATs are able to connect to each other through a
// on-boarded relay.

func TestConnectivityViaOnboardedRelay1(t *testing.T) {
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	relay, stop := helper.CreateNode(ctx, "relay", []string{defaultNetwork}, enableV6)
	defer stop()
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start derp relay and onboard the relay as well
	helper.runNexd(ctx, relay, "--username", username, "--password", password, "relayderp",
		"--hostname custom.relay.nexodus.io", "--certmode manual", "--certdir /.certs/", "--onboard")

	// validate nexd has started on the relay node
	err := helper.nexdStatus(ctx, relay)
	require.NoError(err)

	// Start both the nodes in the relay-only mode to mimic the symmetric NAT scenario
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--relay-only")
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "--relay-only")

	// v4 relay connectivity checks
	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)
}

// TestConnectivityViaOnboardedRelay2 validates the scenario where one peer is
// behind symmetric NATs and other behind reflexive address, are able to
// connect to each other through a on-boarded relay.
func TestConnectivityViaOnboardedRelay2(t *testing.T) {
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	relay, stop := helper.CreateNode(ctx, "relay", []string{defaultNetwork}, enableV6)
	defer stop()
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// start derp relay and onboard the relay as well
	helper.runNexd(ctx, relay, "--username", username, "--password", password, "relayderp",
		"--hostname custom.relay.nexodus.io", "--certmode manual", "--certdir /.certs/", "--onboard")

	// validate nexd has started on the relay node
	err := helper.nexdStatus(ctx, relay)
	require.NoError(err)

	// Start one node with relay-only to mimic symmetric NAT and other behind reflexive address
	helper.runNexd(ctx, node1, "--username", username, "--password", password)
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "--relay-only")

	// v4 relay connectivity checks
	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)
}

func TestConnectivityWithRelaySwitchover(t *testing.T) {
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	pubrelay, stop := helper.CreateNode(ctx, "public-relay", []string{defaultNetwork}, enableV6)
	defer stop()
	onboardrelay, stop := helper.CreateNode(ctx, "onboard-relay", []string{defaultNetwork}, enableV6)
	defer stop()

	// start derp relay and onboard the relay as well
	helper.runNexd(ctx, pubrelay, "--username", username, "--password", password, "relayderp",
		"--hostname relay.nexodus.io", "--certmode manual", "--certdir /.certs/")

	relayIp, err := pubrelay.ContainerIP(ctx)
	require.NoError(err)

	// Set the relay IP as an environment variable so createNode can use it to
	// set the DNS entry pointing to the relay.
	os.Setenv("NEX_DERP_RELAY_IP", relayIp)
	require.NoError(err)

	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	// Start one node with relay-only to mimic symmetric NAT and other behind reflexive address

	helper.runNexd(ctx, node1, "--username", username, "--password", password, "--relay-only")
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "--relay-only")

	// v4 relay connectivity checks
	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)
	node2IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node2)
	require.NoError(err)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// Stop the public relay
	d := 5 * time.Second
	err = pubrelay.Stop(ctx, &d)
	require.NoError(err)

	// wait for the stop, to avoid any flakes
	time.Sleep(5 * time.Second)

	// Ping should fail and return an error
	helper.Logf("Pinging %s from node2", node1IP)
	err = pingWithoutRetry(ctx, node2, inetV4, node1IP)
	require.Error(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = pingWithoutRetry(ctx, node1, inetV4, node2IP)
	require.Error(err)

	// start derp relay and onboard the relay as well
	helper.runNexd(ctx, onboardrelay, "--username", username, "--password", password, "relayderp",
		"--hostname relay.nexodus.io", "--certmode manual", "--certdir /.certs/", "--onboard")

	// validate nexd has started on the relay node
	err = helper.nexdStatus(ctx, onboardrelay)
	require.NoError(err)

	// wait for the relay to get onboard, and peers to be aware of the relay
	time.Sleep(10 * time.Second)

	helper.Logf("Pinging %s from node2", node1IP)
	err = ping(ctx, node2, inetV4, node1IP)
	require.NoError(err)

	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)
}
