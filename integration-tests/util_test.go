//go:build integration
// +build integration

package integration_tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/coreos/go-oidc"
	"github.com/docker/docker/api/types/network"
	"github.com/redhat-et/apex/internal/client"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"golang.org/x/oauth2"
)

// CreateNode creates a container
func (suite *ApexIntegrationSuite) CreateNode(ctx context.Context, name string, networks []string) testcontainers.Container {
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
	}
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType:     providerType,
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(suite.T(), err)
	return ctr
}

func newClient(ctx context.Context, token string) (*client.Client, error) {
	return client.NewClient(ctx, "http://api.apex.local", client.WithToken(token))
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
		cidr := strings.Fields(string(output))[2]
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

func getToken(ctx context.Context, username, password string) (string, error) {
	provider, err := oidc.NewProvider(ctx, "https://auth.apex.local/realms/apex")
	if err != nil {
		return "", err
	}
	config := oauth2.Config{
		ClientID: "apex-cli",
		//ClientSecret: "dhEN2dsqyUg5qmaDAdqi4CmH",
		Endpoint: provider.Endpoint(),
		Scopes:   []string{"openid", "profile", "email"},
	}

	token, err := config.PasswordCredentialsToken(ctx, username, password)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(token)
	if err != nil {
		return "", err
	}

	var rawToken map[string]interface{}
	if err := json.Unmarshal(data, &rawToken); err != nil {
		return "", err
	}

	rawToken["id_token"] = token.Extra("id_token")

	data, err = json.Marshal(rawToken)
	if err != nil {
		return "", err
	}

	return string(data), err
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
