//go:build integration
// +build integration

package integration_tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/redhat-et/apex/internal/client"
	"github.com/stretchr/testify/require"
)

const (
	controller   = "http://localhost:8080"
	clientId     = "api-clients"
	clientSecret = "cvXhCRXI2Vld244jjDcnABCMrTEq2rwE"
)

func healthcheck() error {
	res, err := http.Get(fmt.Sprintf("%s/api/health", controller))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("got %d, wanted 200", res.StatusCode)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if !strings.Contains(string(body), "ok") {
		return fmt.Errorf("service is not healthy")
	}
	return nil
}

func (suite *ApexIntegrationSuite) CreateNode(name string, args []string) *dockertest.Resource {
	options := &dockertest.RunOptions{
		Repository: "quay.io/apex/test",
		Tag:        "ubuntu",
		Tty:        true,
		Name:       name,
		Cmd:        args,
		CapAdd: []string{
			"SYS_MODULE",
			"NET_ADMIN",
			"NET_RAW",
		},
		ExtraHosts: []string{
			"host.docker.internal:host-gateway",
		},
	}
	hostConfig := func(config *docker.HostConfig) {
		//config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	}
	node, err := suite.pool.RunWithOptions(options, hostConfig)
	require.NoError(suite.T(), err)
	err = node.Expire(60)
	require.NoError(suite.T(), err)
	return node
}

func GetToken(username, password string) (string, error) {
	v := url.Values{}
	v.Set("username", username)
	v.Set("password", password)
	v.Set("client_id", clientId)
	v.Set("client_secret", clientSecret)
	v.Set("grant_type", "password")

	res, err := http.PostForm(fmt.Sprintf("%s/auth/realms/controller/protocol/openid-connect/token", controller), v)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", err
	}
	var r map[string]interface{}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	token, ok := r["access_token"]
	if !ok {
		return "", fmt.Errorf("no access token in reponse")
	}
	return token.(string), nil
}

func newClient(token string) (client.Client, error) {
	auth := client.NewTokenAuthenticator(token)
	client, err := client.NewClient(controller, auth)
	if err != nil {
		return client, err
	}
	return client, nil
}

func getWg0IP(ctx context.Context, container *dockertest.Resource) (string, error) {
	var cidr string
	err := backoff.Retry(func() error {
		stdout := new(bytes.Buffer)
		stderr := new(bytes.Buffer)
		code, err := container.Exec(
			[]string{"ip", "--brief", "-4", "address", "show", "wg0"},
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
	var execOut string
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

	return execOut, err
}

// CreateNetwork creates a docker test network
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
