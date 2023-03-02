//go:build integration
// +build integration

package integration_tests

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types/network"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

func findParentDirWhere(directory string, conditional func(fileName string) bool) (string, error) {
	for {
		if conditional(directory) {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("not found")
		}
		directory = parent
	}
}

func findCertsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir, err = findParentDirWhere(dir, func(dir string) bool {
		file := filepath.Join(dir, ".certs")
		f, err := os.Stat(file)
		if err == nil && f.IsDir() {
			return true
		}
		return false
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".certs"), nil
}

type FnConsumer struct {
	Apply func(l testcontainers.Log)
}

func (c FnConsumer) Accept(l testcontainers.Log) {
	c.Apply(l)
}

// CreateNode creates a container
func (suite *ApexIntegrationSuite) CreateNode(ctx context.Context, name string, networks []string) testcontainers.Container {

	certsDir, err := findCertsDir()
	require.NoError(suite.T(), err)

	req := testcontainers.ContainerRequest{
		Image:    "quay.io/apex/test:ubuntu",
		Name:     name,
		Networks: networks,
		CapAdd: []string{
			"SYS_MODULE",
			"NET_ADMIN",
			"NET_RAW",
		},
		ExtraHosts: []string{
			fmt.Sprintf("apex.local:%s", hostDNSName),
			fmt.Sprintf("api.apex.local:%s", hostDNSName),
			fmt.Sprintf("auth.apex.local:%s", hostDNSName),
		},
		AutoRemove: true,
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
	require.NoError(suite.T(), err)

	// wait for the CA cert to get imported.
	wg := sync.WaitGroup{}
	wg.Add(1)
	ctr.FollowOutput(FnConsumer{
		Apply: func(l testcontainers.Log) {
			text := strings.TrimRightFunc(string(l.Content), unicode.IsSpace)
			if text == "ready" {
				wg.Done()
				ctr.StopLogProducer()
			}
		},
	})
	ctr.StartLogProducer(ctx)
	wg.Wait()
	return ctr
}

func newClient(ctx context.Context, username, password string) (*client.Client, error) {
	return client.NewClient(ctx, "http://api.apex.local", nil, client.WithPasswordGrant(username, password))
}

func getContainerIfaceIP(ctx context.Context, dev string, ctr testcontainers.Container) (string, error) {
	var ip string
	err := backoff.Retry(func() error {
		code, outputRaw, err := ctr.Exec(
			ctx,
			[]string{"ip", "--brief", "-4", "address", "show", dev},
		)
		if err != nil {
			return err
		}
		output, err := io.ReadAll(outputRaw)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("exit code %d. output: %s", code, string(output))
		}
		fields := strings.Fields(string(output))
		if len(fields) < 3 {
			return fmt.Errorf("Interface %s has no IP address", dev)
		}
		cidr := fields[2]
		if err != nil {
			return err
		}
		ipAddr, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
		ip = ipAddr.String()
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx))
	return ip, err
}

func ping(ctx context.Context, ctr testcontainers.Container, address string) error {
	err := backoff.Retry(func() error {
		code, outputRaw, err := ctr.Exec(
			ctx,
			[]string{"ping", "-c", "2", "-w", "2", address},
		)
		if err != nil {
			return err
		}
		output, err := io.ReadAll(outputRaw)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("exit code %d. stdout: %s", code, string(output))
		}
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx))
	return err
}

// containerExec exec container commands
func (suite *ApexIntegrationSuite) containerExec(ctx context.Context, container testcontainers.Container, cmd []string) (string, error) {
	code, outputRaw, err := container.Exec(
		ctx,
		cmd,
	)
	if err != nil {
		return "", err
	}
	nodeName, _ := container.Name(ctx)
	if cmd[0] != "/bin/apexd" {
		suite.logger.Infof("Running command on %s: %s", nodeName, strings.Join(cmd, " "))
	}
	output, err := io.ReadAll(outputRaw)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("exit code %d. stderr: %s", code, string(output))
	}

	return string(output), err
}

// CreateNetwork creates a docker network
func (suite *ApexIntegrationSuite) CreateNetwork(ctx context.Context, name, cidr string) testcontainers.Network {
	req := testcontainers.GenericNetworkRequest{
		ProviderType: providerType,
		NetworkRequest: testcontainers.NetworkRequest{
			Name:   name,
			Driver: "bridge",
			IPAM: &network.IPAM{
				Driver: ipamDriver,
				Config: []network.IPAMConfig{
					{
						Subnet: cidr,
					},
				},
			},
		},
	}
	net, err := testcontainers.GenericNetwork(
		ctx,
		req,
	)
	require.NoError(suite.T(), err)
	return net
}

// lineCount for validating peer counts
func lineCount(s string) (int, error) {
	r := bufio.NewReader(strings.NewReader(s))
	count := 0
	for {
		_, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		count++
	}

	return count, nil
}

func (suite *ApexIntegrationSuite) runApex(ctx context.Context, node testcontainers.Container, args ...string) {
	cmd := []string{"/bin/apexd"}
	cmd = append(cmd, args...)
	cmd = append(cmd, "https://apex.local")
	nodeName, _ := node.Name(ctx)
	out, err := suite.containerExec(ctx, node, cmd)
	if suite.T().Failed() {
		suite.logger.Errorf("execution of command on %s failed: %s", nodeName, strings.Join(cmd, " "))
		suite.logger.Errorf("output:\n%s", out)
		suite.logger.Errorf("%+v", err)
	}
}

func networkAddr(n *net.IPNet) net.IP {
	network := net.ParseIP("0.0.0.0").To4()
	for i := 0; i < len(n.IP); i++ {
		network[i] = n.IP[i] & n.Mask[i]
	}
	return network
}

// wgDump dump wg sessions for failed test debugging
func (suite *ApexIntegrationSuite) wgDump(ctx context.Context, container testcontainers.Container) string {
	wgSpokeShow, err := suite.containerExec(ctx, container, []string{"wg", "show", "wg0", "dump"})
	if err != nil {
		return ""
	}

	return wgSpokeShow
}

// routesDump dump routes for failed test debugging
func (suite *ApexIntegrationSuite) routesDump(ctx context.Context, container testcontainers.Container) string {
	wgSpokeShow, err := suite.containerExec(ctx, container, []string{"ip", "route"})
	if err != nil {
		return ""
	}

	return wgSpokeShow
}

// gatherFail gather details on a failed test for debugging
func (suite *ApexIntegrationSuite) gatherFail(ctx context.Context, containers ...testcontainers.Container) string {
	var gatherOut []string

	for _, c := range containers {
		ip, _ := getContainerIfaceIP(ctx, "wg0", c)
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s wg0 IP:\n %s, ", nodeName, ip)
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		ip, _ := getContainerIfaceIP(ctx, "eth0", c)
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s eth0 IP:\n %s, ", nodeName, ip)
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s wg-dump:\n %s, ", nodeName, suite.wgDump(ctx, c))
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s routes:\n %s, ", nodeName, suite.routesDump(ctx, c))
		gatherOut = append(gatherOut, routes)
	}

	return strings.Join(gatherOut, "\n")
}

// runCommand runs the cmd and returns the combined stdout and stderr
func (suite *ApexIntegrationSuite) runCommand(cmd ...string) (string, error) {
	suite.logger.Infof("Running command: %s", strings.Join(cmd, " "))
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %s (%s)", strings.Join(cmd, " "), err, output)
	}

	return string(output), nil
}
