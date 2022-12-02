//go:build integration
// +build integration

package integration_tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/coreos/go-oidc"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/redhat-et/apex/internal/client"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// CreateNode creates a container
func (suite *ApexIntegrationSuite) CreateNode(name, network string, args []string) *dockertest.Resource {
	options := &dockertest.RunOptions{
		Repository: "quay.io/apex/test",
		Tag:        "ubuntu",
		Tty:        true,
		Name:       name,
		NetworkID:  network,
		Cmd:        args,
		CapAdd: []string{
			"SYS_MODULE",
			"NET_ADMIN",
			"NET_RAW",
		},
		ExtraHosts: []string{
			"apex.local:host-gateway",
			"api.apex.local:host-gateway",
			"auth.apex.local:host-gateway",
		},
	}
	hostConfig := func(config *docker.HostConfig) {
		// config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	}
	node, err := suite.pool.RunWithOptions(options, hostConfig)
	require.NoError(suite.T(), err)
	err = node.Expire(120)
	require.NoError(suite.T(), err)
	return node
}

func newClient(ctx context.Context, token string) (*client.Client, error) {
	return client.NewClient(ctx, "http://api.apex.local", client.WithToken(token))
}

func getContainerIfaceIP(ctx context.Context, dev string, container *dockertest.Resource) (string, error) {
	var cidr string
	err := backoff.Retry(func() error {
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		code, err := container.Exec(
			[]string{"ip", "--brief", "-4", "address", "show", dev},
			dockertest.ExecOptions{
				StdOut: stdout,
				StdErr: stderr,
			},
		)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("exit code %d. stderr: %s", code, stderr.String())
		}
		cidr = strings.Fields(stdout.String())[2]
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx))
	if err != nil {
		return "", err
	}
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	return ip.String(), err
}

func ping(ctx context.Context, container *dockertest.Resource, address string) error {
	err := backoff.Retry(func() error {
		stdout := new(bytes.Buffer)
		code, err := container.Exec(
			[]string{"ping", "-c", "2", "-w", "2", address}, dockertest.ExecOptions{
				StdOut: stdout,
			},
		)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("exit code %d. stdout: %s", code, stdout.String())
		}
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx))
	return err
}

// containerExec TODO: this will be for deleting keys, restarting apex and creating general chaos
func containerExec(ctx context.Context, container *dockertest.Resource, cmd []string) (string, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code, err := container.Exec(
		cmd,
		dockertest.ExecOptions{
			StdOut: stdout,
			StdErr: stderr,
		},
	)
	if code != 0 {
		return "", fmt.Errorf("exit code %d. stderr: %s", code, stderr.String())
	}
	if err != nil {
		return "", err
	}

	return stdout.String(), err
}

func getToken(ctx context.Context, username, password string) (string, error) {
	provider, err := oidc.NewProvider(ctx, "http://auth.apex.local")
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
func (suite *ApexIntegrationSuite) CreateNetwork(name, cidr string) *dockertest.Network {
	net, err := suite.pool.CreateNetwork(name, func(config *docker.CreateNetworkOptions) {
		config.Driver = "bridge"
		config.IPAM = &docker.IPAMOptions{
			Driver: "default",
			Config: []docker.IPAMConfig{
				{
					Subnet: cidr,
				},
			},
		}
	})
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
