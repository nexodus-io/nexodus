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
// 2. Curl an Internet address from the exit node client to ensure connectivity via the origin server.
// 3. Kill nexd on the origin server.
// 4. Negative test from the exit node client to ensure connectivity is unavailable origin server.
// 5. Use nexctl to disable exit node client configuration since the origin server is no longer available.
// 6. Curl an Internet address from the exit node client to ensure connectivity is restored.
// +------------------------+                   +------------------------+                 +------------------------+
// |               wg0/eth0 |    default-net    | eth0/wg0               |   default-net   |        Internet        |
// | nexd-exit-node-client  |===================| nexd-exit-node-server  |=================|                        |
// +------------------------+                   +------------------------+                 +------------------------+
func TestExitNode(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	nexExitNodeClient, stop := helper.CreateNodePrivileged(ctx, "exit-client", []string{defaultNetwork}, enableV6)
	defer stop()
	// create nodes with two interfaces, one in the default network and one in the site2-net
	nexExitNodeServer, stop := helper.CreateNodePrivileged(ctx, "exit-server", []string{defaultNetwork}, enableV6)
	defer stop()

	helper.runNexd(ctx, nexExitNodeServer,
		"--username", username,
		"--password", password,
		"router",
		"--exit-node")

	// TODO: replace the sleep with a retry of the command 'nexctl --output=json nexd exit-node list' until an exit node is present
	time.Sleep(time.Second * 20)

	helper.runNexd(ctx, nexExitNodeClient,
		"--username", username,
		"--password", password,
		"--exit-node-client")

	nexRouterSite1IP, err := getContainerIfaceIP(ctx, inetV4, "eth0", nexExitNodeClient)
	require.NoError(err)

	helper.Logf("Curling 1.1.1.1 from nexRouterSite1 %s", nexRouterSite1IP)
	curlOut, err := helper.containerExec(ctx, nexExitNodeClient, []string{"curl", "--silent", "--show-error", "-m", "10", "1.1.1.1"})
	require.NoError(err)
	helper.Logf("exit node client curl output: %s", curlOut)

	// Kill nexodus on the exit node server
	_, err = helper.containerExec(ctx, nexExitNodeServer, []string{"killall", "nexd"})
	require.NoError(err)

	// Negative test since the exit node client is now orphaned, the curl should fail since the exit node is no longer available
	helper.Logf("Curling 1.1.1.1 from nexRouterSite1 %s (should fail)", nexRouterSite1IP)
	_, err = helper.containerExec(ctx, nexExitNodeClient, []string{"curl", "--silent", "--show-error", "-m", "10", "1.1.1.1"})
	require.Error(err)

	// Disable the exit node client on the node to return the normal traffic flow
	disableOut, err := helper.containerExec(ctx, nexExitNodeClient, []string{"/bin/nexctl", "nexd", "exit-node", "disable"})
	require.NoError(err)
	require.Contains(disableOut, "Success")

	// Sleep for 1s to ensure the routing and nftable rules are removed before the next test (likely unnecessary but there to avoid a race condition/flake)
	time.Sleep(time.Second * 1)
	helper.Logf("Curling 1.1.1.1 from nexRouterSite1 %s (should succeed)", nexRouterSite1IP)
	curlOut, err = helper.containerExec(ctx, nexExitNodeClient, []string{"curl", "--silent", "--show-error", "-m", "10", "1.1.1.1"})
	helper.Logf("exit node client curl output: %s", curlOut)
	require.NoError(err)
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
		},
		Mounts: []testcontainers.ContainerMount{
			{
				Source: testcontainers.GenericBindMountSource{
					HostPath: certsDir,
				},
				Target:   "/.certs",
				ReadOnly: true,
			},
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
