//go:build integration

package integration_tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/models"
	"github.com/testcontainers/testcontainers-go"
)

// TestSecurityGroups validate that Security Groups.
// All tests will use netcat to open a listening port that will echo the hostname of the
// device to anything that connects to the listening port. To test that device, a connection
// is attempted to the listening device and verifies it received the hostname of the listener.
// If the return results contain the hostname of the listener, the connection was successful.
// If the return results are empty, then the connection was unsuccessful.
// Since there is a period of time between the request of the policy from the nexctl api
// call and the execution of the policy, a retry helper will validate the nftable rules have
// changed before the tests are performed. Tests will verify combinations of UDP, TCP, IPv4
// and IPv6.
// Tests performed are as follows:
// 1. The security group IDs for the nodes are verified present and org ID is gathered.
// 2. This section will validate the default policy is allowing port traffic to ports.
// 3. The second set will add a policy and verify the explicitly allowed ports are reachable.
// 4. The next section will perform negative tests that will add a new policy and a netcat
// listener will be put in place on a port that is not in the policy allow and verify that
// connection is not permitted.
// 5. Test scopes by creating a new user and performing negative tests.
// 6. Validate security group creation and deletion.
// 7. Negative testing to verify sanity checks of [ Port, IP_Ranges, Protocols ] are functional.
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

	helper.runNexd(ctx, node2, "--username", username, "--password", password)
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

	deviceMap := map[string]models.Device{}
	for _, device := range devices {
		deviceMap[device.Hostname] = device
	}
	require.Equal(len(deviceMap), 2)
	secGroupID := deviceMap[node1Hostname].SecurityGroupId.String()
	orgID := deviceMap[node1Hostname].OrganizationID.String()
	require.Equal(secGroupID, deviceMap[node2Hostname].SecurityGroupId.String())

	node1IPv4 := deviceMap[node1Hostname].TunnelIP
	node1IPv6 := deviceMap[node1Hostname].TunnelIpV6
	node2IPv4 := deviceMap[node2Hostname].TunnelIP
	node2IPv6 := deviceMap[node2Hostname].TunnelIpV6

	// v4 TCP port 11114 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv4, protoTCP, "11114")
	require.NoError(err)
	connectResults, err := helper.connectToPort(ctx, node2, node1IPv4, protoTCP, "11114")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v4 UDP port 11124 should succeed
	err = helper.startPortListener(ctx, node2, node2IPv4, protoUDP, "11124")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node1, node2IPv4, protoUDP, "11124")
	require.NoError(err)
	require.Equal(node2Hostname, connectResults)

	// v6 TCP port 11116 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoTCP, "11116")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoTCP, "11116")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v6 UDP port 11126 should succeed
	err = helper.startPortListener(ctx, node2, node2IPv6, protoUDP, "11126")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node1, node2IPv6, protoUDP, "11126")
	require.NoError(err)
	require.Equal(node2Hostname, connectResults)

	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err := helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.NotEmpty(nfOutBefore)

	// Update the security group with the new inbound and outbound rules
	inboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "18", "25", []string{"100.64.0.0/10"}),
		helper.createSecurityRule("tcp", "18", "25", []string{"200::/64"}),
	}
	outboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "0", "0", []string{""}),
	}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	allSucceeded, err := helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
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
	require.NotEmpty(nfOutBefore)

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

	allSucceeded, err = helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v6 tcp 1111 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoTCP, "1111")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoTCP, "1111")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v4 tcp 1122 should fail
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
	require.NotEmpty(nfOutBefore)

	// create the new inbound and outbound rules
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("icmpv4", "0", "0", []string{"100.64.0.1-100.127.0.50"}),
		helper.createSecurityRule("icmpv6", "0", "0", []string{""}),
	}
	outboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("icmpv4", "0", "0", []string{""}),
		helper.createSecurityRule("icmpv6", "0", "0", []string{"200::1-200::ffff:ffff:ffff:ffff"}),
	}

	// update the security group with the new inbound and outbound rules
	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	allSucceeded, err = helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
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

	// Test Proto: ipv4/ipv6 Port: x-y Range: 100.64.0.0/10 & 0200::/8
	nfOutBefore, err = helper.containerExec(ctx, node1, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.NotEmpty(nfOutBefore)

	// create the new inbound and outbound rules
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "100", "200", []string{"100.64.0.0/10"}),
		helper.createSecurityRule("ipv6", "300", "400", []string{"200::/64"}),
	}
	outboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "0", "0", []string{"100.64.0.0/10"}),
		helper.createSecurityRule("ipv6", "0", "0", []string{"200::/64"}),
	}

	// update the security group with the new inbound and outbound rules
	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	allSucceeded, err = helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v4 tcp 150 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv4, protoTCP, "150")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv4, protoTCP, "150")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v4 udp 160 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv4, protoUDP, "160")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv4, protoUDP, "160")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v6 tcp 350 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoTCP, "350")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoTCP, "350")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v6 udp 360 should succeed
	err = helper.startPortListener(ctx, node1, node1IPv6, protoUDP, "360")
	require.NoError(err)
	connectResults, err = helper.connectToPort(ctx, node2, node1IPv6, protoUDP, "360")
	require.NoError(err)
	require.Equal(node1Hostname, connectResults)

	// v4 tcp 12345 should fail
	err = helper.startPortListener(ctx, node1, node1IPv4, protoTCP, "12345")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node2, node1IPv4, protoTCP, "12345")
	require.Empty(connectResults)

	// v6 udp 54321 should fail
	err = helper.startPortListener(ctx, node1, node1IPv6, protoUDP, "54321")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node2, node1IPv6, protoUDP, "54321")
	require.Empty(connectResults)
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

	deviceMap := map[string]models.Device{}
	for _, device := range devices {
		deviceMap[device.Hostname] = device
	}
	require.Equal(len(deviceMap), 2)
	secGroupID := deviceMap[node1Hostname].SecurityGroupId.String()
	orgID := deviceMap[node1Hostname].OrganizationID.String()
	require.Equal(secGroupID, deviceMap[node2Hostname].SecurityGroupId.String())

	// register the v4 and v6 addresses for both devices
	node1IPv4 := deviceMap[node1Hostname].TunnelIP
	node1IPv6 := deviceMap[node1Hostname].TunnelIpV6
	node2IPv4 := deviceMap[node2Hostname].TunnelIP
	node2IPv6 := deviceMap[node2Hostname].TunnelIpV6

	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err := helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.NotEmpty(nfOutBefore)

	// Update the security group with the new inbound and outbound rules
	inboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("udp", "5000", "6000",
			[]string{"100.64.0.0/10",
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
	allSucceeded, err := helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
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

	// Only change one octet of one address in the inbound prefix field and ensure nftables update (s/172.28.100.100/172.28.100.101/)
	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err = helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.NotEmpty(nfOutBefore)

	// Update the security group with the new inbound and outbound rules
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("udp", "5000", "6000",
			[]string{"100.64.0.0/10",
				"172.28.100.1-172.28.100.101",
				"192.168.168.100",
			}),
		helper.createSecurityRule("udp", "0", "0",
			[]string{"200::/64",
				"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db8:ffff:ffff:ffff:ffff:ffff:ffff",
				"2001:0000:0000:0000:0000:0000:0000:0010",
			}),
	}
	// Verify an empty rule will apply a permit all
	outboundRules = []public.ModelsSecurityRule{}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	allSucceeded, err = helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// manually delete the netfilter table to ensure it is recreated when the next group is applied
	_, err = helper.containerExec(ctx, node2, []string{"nft", "flush", "ruleset"})
	require.NoError(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	nfOutBefore, err = helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.Empty(nfOutBefore)

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

	allSucceeded, err = helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
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

	// The next section tests scopes, start by creating a new user and running negative tests
	username2, cleanup2 := helper.createNewUser(ctx, password)
	defer cleanup2()

	_, err = helper.runCommand(nexctl,
		"--username", username2,
		"--password", password,
		"security-group", "list",
		"--organization-id", orgID,
	)
	require.Error(err)
	require.ErrorContains(err, "404")

	_, err = helper.runCommand(nexctl,
		"--username", username2,
		"--password", password,
		"security-group", "delete",
		"--organization-id", orgID,
		"--security-group-id", secGroupID,
	)
	require.Error(err)

	// gather the nftables from both nodes to verify the new rules are applied before testing and block until the rules are applied or fail if max attempts is reached
	nfOutBefore, err = helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)

	// delete the security group and ensure the device updates it's netfilter rules to fall back to a default where no group is defined
	sgDel, err := helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"security-group", "delete",
		"--organization-id", orgID,
		"--security-group-id", secGroupID,
	)
	require.NoError(err)
	require.Contains(sgDel, secGroupID)

	allSucceeded, err = helper.retryNftCmdOnAllNodes(ctx, []testcontainers.Container{node1, node2}, []string{"nft", "list", "ruleset"}, nfOutBefore)
	require.NoError(err)
	require.True(allSucceeded)

	// v4 UDP 9000 should succeed now that the security group has been deleted, the netfilter table should be cleared as a result
	err = helper.startPortListener(ctx, node1, node1IPv4, protoUDP, "9000")
	require.NoError(err)
	connectResults, _ = helper.connectToPort(ctx, node2, node1IPv4, protoUDP, "9000")
	require.Equal(node1Hostname, connectResults)

	// create the new inbound and outbound rules
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "0", "0", []string{}),
		helper.createSecurityRule("icmpv4", "0", "0", []string{}),
	}
	outboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("tcp", "0", "0", []string{}),
		helper.createSecurityRule("icmpv4", "0", "0", []string{}),
	}
	// Marshal rules to JSON
	inboundJSON, err := json.Marshal(inboundRules)
	require.NoError(err)
	outboundJSON, err := json.Marshal(outboundRules)
	require.NoError(err)
	_, err = helper.runCommand(nexctl,
		"--username", username2,
		"--password", password,
		"security-group", "create",
		"--name", "test-create-group",
		"--description", "test create group sg_e2e_extended",
		"--organization-id", orgID,
		"--inbound-rules", string(inboundJSON),
		"--outbound-rules", string(outboundJSON),
	)
	require.Error(err)

	_, err = helper.runCommand(nexctl,
		"--username", username,
		"--password", password,
		"security-group", "create",
		"--name", "test-create-group",
		"--description", "test create group sg_e2e_extended",
		"--organization-id", orgID,
		"--inbound-rules", string(inboundJSON),
		"--outbound-rules", string(outboundJSON),
	)
	require.NoError(err)

	// Negative test where the from_port is greater than the to_port
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("udp", "456", "123",
			[]string{"100.64.0.0/10",
				"172.28.100.1-172.28.100.100",
				"192.168.168.100",
			}),
		helper.createSecurityRule("udp", "0", "0",
			[]string{"200::/64",
				"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db8:ffff:ffff:ffff:ffff:ffff:ffff",
				"2001:0000:0000:0000:0000:0000:0000:0010",
			}),
	}
	outboundRules = []public.ModelsSecurityRule{}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.Error(err)

	// Negative test where the from_port is 0 and not a valid 1-65535
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("udp", "0", "123",
			[]string{"100.64.0.0/10",
				"172.58.100.1-172.28.100.100",
			}),
		helper.createSecurityRule("ipv6", "0", "0",
			[]string{"200::/60",
				"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db6:ffff:ffff:ffff:ffff:ffff:ffff",
				"2001:0000:0000:0000:0000:0000:0000:0020",
			}),
	}
	outboundRules = []public.ModelsSecurityRule{}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.Error(err)

	// Negative test where there is an invalid address in ip_ranges
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("udp", "123", "456",
			[]string{"100.64.0.0/10",
				"172.28.100.1-172.28.100.100",
				"MEOWDY_PARTNER",
			}),
		helper.createSecurityRule("udp", "0", "0",
			[]string{"200::/64",
				"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db8:ffff:ffff:ffff:ffff:ffff:ffff",
				"2001:0000:0000:0000:0000:0000:0000:0010",
			}),
	}
	outboundRules = []public.ModelsSecurityRule{}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.Error(err)

	// Negative test where there is an invalid protocol specified
	inboundRules = []public.ModelsSecurityRule{
		helper.createSecurityRule("udp", "123", "456",
			[]string{"100.64.0.0/10",
				"172.28.100.1-172.28.100.100",
				"192.168.168.100",
			}),
		helper.createSecurityRule("ipv9", "0", "0",
			[]string{"200::/64",
				"2003:0db8:0000:0000:0000:0000:0000:0000-2003:0db8:ffff:ffff:ffff:ffff:ffff:ffff",
				"2001:0000:0000:0000:0000:0000:0000:0010",
			}),
	}
	outboundRules = []public.ModelsSecurityRule{}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.Error(err)
}

// TestSecurityGroupProtocolsOnly tests rule entry without error only, for
// all combinations of an explicit protocol and wildcard port and ip_range
func TestSecurityGroupProtocolsOnly(t *testing.T) {
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

	deviceMap := map[string]models.Device{}
	for _, device := range devices {
		deviceMap[device.Hostname] = device
	}
	require.Equal(len(deviceMap), 2)
	secGroupID := deviceMap[node1Hostname].SecurityGroupId.String()
	orgID := deviceMap[node1Hostname].OrganizationID.String()
	require.Equal(secGroupID, deviceMap[node2Hostname].SecurityGroupId.String())

	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err := helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.NotEmpty(nfOutBefore)

	// Test all accepted protocols and a null and empty string
	inboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "0", "0", []string{""}),
		helper.createSecurityRule("ipv6", "0", "0", []string{}),
		helper.createSecurityRule("tcp", "0", "0", []string{""}),
		helper.createSecurityRule("udp", "0", "0", []string{}),
		helper.createSecurityRule("icmpv4", "0", "0", []string{""}),
		helper.createSecurityRule("icmpv6", "0", "0", []string{}),
	}
	outboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "0", "0", []string{""}),
		helper.createSecurityRule("ipv6", "0", "0", []string{}),
		helper.createSecurityRule("tcp", "0", "0", []string{""}),
		helper.createSecurityRule("udp", "0", "0", []string{}),
		helper.createSecurityRule("icmp", "0", "0", []string{""}),
	}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)
}

// TestSecurityGroupProtocolsPortsOnly tests rule entry without error only, for
// all combinations of an explicit protocol and ports with a wildcard ip_range
func TestSecurityGroupProtocolsPortsOnly(t *testing.T) {
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

	deviceMap := map[string]models.Device{}
	for _, device := range devices {
		deviceMap[device.Hostname] = device
	}
	require.Equal(len(deviceMap), 2)
	secGroupID := deviceMap[node1Hostname].SecurityGroupId.String()
	orgID := deviceMap[node1Hostname].OrganizationID.String()
	require.Equal(secGroupID, deviceMap[node2Hostname].SecurityGroupId.String())

	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err := helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.NotEmpty(nfOutBefore)

	// Test all accepted protocols that accept a port range of 1-65535
	inboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "1000", "1100", []string{""}),
		helper.createSecurityRule("ipv6", "2000", "2100", []string{}),
		helper.createSecurityRule("tcp", "3000", "3100", []string{""}),
		helper.createSecurityRule("udp", "4000", "4100", []string{}),
	}
	outboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "1300", "1300", []string{""}),
		helper.createSecurityRule("ipv6", "2300", "2300", []string{}),
		helper.createSecurityRule("tcp", "3300", "3300", []string{""}),
		helper.createSecurityRule("udp", "4300", "4300", []string{}),
	}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)
}

// TestSecurityGroupProtocolsPortsCIDR tests rule entry without error only, for
// all combinations of an explicit protocol, ports and ip_range e.g., no wildcards
func TestSecurityGroupProtocolsPortsCIDR(t *testing.T) {
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

	deviceMap := map[string]models.Device{}
	for _, device := range devices {
		deviceMap[device.Hostname] = device
	}
	require.Equal(len(deviceMap), 2)
	secGroupID := deviceMap[node1Hostname].SecurityGroupId.String()
	orgID := deviceMap[node1Hostname].OrganizationID.String()
	require.Equal(secGroupID, deviceMap[node2Hostname].SecurityGroupId.String())

	// gather the nftables before the new rules are applied to check against the new rules created next
	nfOutBefore, err := helper.containerExec(ctx, node2, []string{"nft", "list", "ruleset"})
	require.NoError(err)
	require.NotEmpty(nfOutBefore)

	// Test all accepted protocols, port range and the various accepted ip_range formats
	inboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "5000", "5999", []string{"10.130.0.1-10.130.0.5", "192.168.64.10-192.168.64.50", "100.100.0.128/25"}),
		helper.createSecurityRule("ipv6", "6000", "6999", []string{"F100:0db8:0000:0000:0000:0000:0000:0000 - F200:0db8:ffff:ffff:ffff:ffff:ffff:ffff"}),
		helper.createSecurityRule("tcp", "7000", "7999", []string{"100.3.2.1/32"}),
		helper.createSecurityRule("udp", "8000", "8999", []string{"100.3.2.1"}),
	}
	outboundRules := []public.ModelsSecurityRule{
		helper.createSecurityRule("ipv4", "1400", "1400", []string{"192.168.64.10-192.168.64.50"}),
		helper.createSecurityRule("ipv6", "2400", "2401", []string{"fd00:face:b00c:cafe::/64", "200::1-200::5", "fd00:face:b00c:cafe::1"}),
		helper.createSecurityRule("tcp", "3400", "3402", []string{"10.130.0.1-10.130.0.5", "192.168.64.10-192.168.64.50", "100.100.0.128/25"}),
		helper.createSecurityRule("udp", "4400", "4403", []string{"F100:0db8:0000:0000:0000:0000:0000:0000 - F200:0db8:ffff:ffff:ffff:ffff:ffff:ffff", "2002:0db8::/64"}),
	}

	err = helper.securityGroupRulesUpdate(username, password, inboundRules, outboundRules, secGroupID, orgID)
	require.NoError(err)
}
