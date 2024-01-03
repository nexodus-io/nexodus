//go:build integration

package integration_tests

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

// TestExitNode performs the following exit node tests:
// 1. Start two nodes, one is an exit node client and the other is an exit node origin/server.
// 2. Create a loopback on the origin server that will only be reachable if tunneling through the exit node.
// 3. Connect from the exit node client to ensure connectivity via the origin server.
// 4. Use nexctl to disable exit node client configuration since the origin server is no longer available.
// 5. Negative test from the exit node client to ensure connectivity is unavailable origin server.
// +------------------------+                   +------------------------+
// |               wg0/eth0 |    default-net    | eth0/wg0     loopback0 |
// | nexd-exit-node-client  |===================| nexd-exit-node-server  |
// +------------------------+                   +------------------------+
func TestExitNode(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()
	webserverIP := "10.40.1.1/32"
	webserver := "10.40.1.1"

	nexExitNodeClient, stop := helper.CreateNodePrivileged(ctx, "exit-client", []string{defaultNetwork}, enableV6)
	defer stop()
	// create nodes with two interfaces, one in the default network and one in the site2-net
	nexExitNodeServer, stop := helper.CreateNodePrivileged(ctx, "exit-server", []string{defaultNetwork}, enableV6)
	defer stop()

	// add a loopback to the exit node server/origin
	_, err := helper.containerExec(ctx, nexExitNodeServer, []string{"ip", "addr", "add", webserverIP, "dev", "lo"})
	require.NoError(err)

	helper.runNexd(ctx, nexExitNodeServer,
		"--username", username,
		"--password", password,
		"router",
		"--exit-node")

	// Since we are starting the exit node client at runtime, we are assuming the exit node route has propagated
	time.Sleep(time.Second * 10)

	helper.runNexd(ctx, nexExitNodeClient,
		"--username", username,
		"--password", password,
		"--exit-node-client")

	nexExitNodeServerHostname, err := helper.getNodeHostname(ctx, nexExitNodeServer)
	require.NoError(err)

	err = helper.startPortListener(ctx, nexExitNodeServer, webserver, protoTCP, "8000")
	require.NoError(err)
	connectResults, err := helper.connectToPort(ctx, nexExitNodeClient, webserver, protoTCP, "8000")
	require.NoError(err)
	require.Equal(nexExitNodeServerHostname, connectResults)

	// Disable the exit node client on the node to return the normal traffic flow
	disableOut, err := helper.containerExec(ctx, nexExitNodeClient, []string{"/bin/nexctl", "nexd", "exit-node", "disable"})
	require.NoError(err)
	require.Contains(disableOut, "Success")

	// Sleep for 1s to ensure the routing and nftable rules are removed before the next test (likely unnecessary but there to avoid a race condition/flake)
	time.Sleep(time.Second * 1)

	// Negative test since the exit node client is no longer in exit mode, the connection should fail since the exit node is no longer available
	err = helper.startPortListener(ctx, nexExitNodeServer, webserver, protoTCP, "8080")
	require.NoError(err)
	_, err = helper.connectToPort(ctx, nexExitNodeClient, webserver, protoTCP, "8080")
	require.Error(err)
}

// CreateNodePrivileged creates a privileged container
func (helper *Helper) CreateNodePrivileged(ctx context.Context, nameSuffix string, networks []string, v6 v6Enable) (testcontainers.Container, func()) {

	// Host modifiers differ for a container for a container with and without v6 enabled
	var hostConfSysctl map[string]string
	if v6 == enableV6 {
		hostConfSysctl = map[string]string{
			"net.ipv6.conf.all.disable_ipv6": "0",
			"net.ipv4.ip_forward":            "1",
			"net.ipv6.conf.all.forwarding":   "1",
		}
	} else {
		hostConfSysctl = map[string]string{
			"net.ipv4.ip_forward":          "1",
			"net.ipv6.conf.all.forwarding": "1",
		}
	}

	// Name containers <test>-<nameSuffix>, where <test> is the name of the calling test
	name := helper.T.Name() + "-" + nameSuffix
	name = strings.ReplaceAll(name, "/", "-")

	certsDir, err := findCertsDir()
	require.NoError(helper.T, err)

	req := testcontainers.ContainerRequest{
		Image:    "quay.io/nexodus/nexd:latest",
		Name:     name,
		Networks: networks,
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.Sysctls = hostConfSysctl
			hostConfig.CapAdd = []string{
				"SYS_MODULE",
				"NET_ADMIN",
				"NET_RAW",
			}
			hostConfig.Privileged = true
			hostConfig.ExtraHosts = []string{
				fmt.Sprintf("try.nexodus.127.0.0.1.nip.io:%s", hostDNSName),
				fmt.Sprintf("api.try.nexodus.127.0.0.1.nip.io:%s", hostDNSName),
				fmt.Sprintf("auth.try.nexodus.127.0.0.1.nip.io:%s", hostDNSName),
			}
			hostConfig.AutoRemove = true
			hostConfig.Binds = append(hostConfig.Binds, certsDir+":/.certs")
		},
		Cmd: []string{
			"/update-ca.sh",
			"/bin/bash",
			"-c",
			"echo ready && sleep infinity",
		},
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType:     providerType,
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(helper.T, err)
	stop := func() {
		if helper.T.Failed() {
			helper.Log(helper.gatherFail(ctr))
		}
	}

	// wait for the CA cert to get imported.
	wg := sync.WaitGroup{}
	wg.Add(1)
	ctr.FollowOutput(FnConsumer{
		Apply: func(l testcontainers.Log) {
			text := strings.TrimRightFunc(string(l.Content), unicode.IsSpace)
			if text == "ready" {
				wg.Done()
				err = ctr.StopLogProducer()
				if err != nil {
					helper.Log("could not stop log producer: %w", err)
				}
			}
		},
	})
	err = ctr.StartLogProducer(ctx)
	helper.require.NoError(err)
	wg.Wait()
	return ctr, stop
}
