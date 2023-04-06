//go:build integration
// +build integration

package integration_tests

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
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

	"golang.org/x/oauth2"

	"github.com/Nerzal/gocloak/v13"
	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/nexodus-io/nexodus/internal/util"
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
		return "", errors.New(fmt.Sprintf("certs directory error: %v, try running 'make cacerts'", err))
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
func (suite *NexodusIntegrationSuite) CreateNode(ctx context.Context, name string, networks []string, v6 v6Enable) testcontainers.Container {

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
			"net.ipv4.ip_forward": "1",
		}
	}

	certsDir, err := findCertsDir()
	require.NoError(suite.T(), err)

	req := testcontainers.ContainerRequest{
		Image:    "quay.io/nexodus/test:ubuntu",
		Name:     name,
		Networks: networks,
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.Sysctls = hostConfSysctl
			hostConfig.CapAdd = []string{
				"SYS_MODULE",
				"NET_ADMIN",
				"NET_RAW",
			}
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

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}
func newClient(ctx context.Context, username, password string) (*client.APIClient, error) {
	return client.NewAPIClient(ctx, "http://api.try.nexodus.127.0.0.1.nip.io", nil, client.WithPasswordGrant(username, password))
}

func getContainerIfaceIP(ctx context.Context, family ipFamily, dev string, ctr testcontainers.Container) (string, error) {
	var ip string
	err := backoff.Retry(func() error {
		code, outputRaw, err := ctr.Exec(
			ctx,
			[]string{"ip", "--brief", family.String(), "address", "show", dev},
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

func ping(ctx context.Context, ctr testcontainers.Container, family ipFamily, address string) error {
	err := backoff.Retry(func() error {
		code, outputRaw, err := ctr.Exec(
			ctx,
			[]string{"ping", family.String(), "-c", "2", "-w", "2", address},
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
func (suite *NexodusIntegrationSuite) containerExec(ctx context.Context, container testcontainers.Container, cmd []string) (string, error) {
	nodeName, _ := container.Name(ctx)
	if cmd[0] != "wg" && cmd[0] != "cat" {
		suite.logger.Infof("Running command on %s: %s", nodeName, strings.Join(cmd, " "))
	}
	code, outputRaw, err := container.Exec(
		ctx,
		cmd,
	)
	if err != nil {
		return "", err
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
func (suite *NexodusIntegrationSuite) CreateNetwork(ctx context.Context, name, cidr string) testcontainers.Network {
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

// runNexd copies the nexd command to a file on the container and runs it piping the logs to a file
func (suite *NexodusIntegrationSuite) runNexd(ctx context.Context, node testcontainers.Container, args ...string) {
	nodeName, _ := node.Name(ctx)
	runScript := fmt.Sprintf("%s-nexd-run.sh", strings.TrimPrefix(nodeName, "/"))
	runScriptLocal := fmt.Sprintf("tmp/%s", runScript)
	cmd := []string{"NEXD_LOGLEVEL=debug", "/bin/nexd"}
	cmd = append(cmd, args...)
	cmd = append(cmd, "https://try.nexodus.127.0.0.1.nip.io")
	cmd = append(cmd, ">> /nexd.logs 2>&1 &")

	// write the nexd run command to a local file
	nexodus.WriteToFile(suite.logger, strings.Join(cmd, " "), runScriptLocal, 0755)
	// copy the nexd run script to the test container
	err := node.CopyFileToContainer(ctx, runScriptLocal, fmt.Sprintf("/bin/%s", runScript), 0755)
	suite.Require().NoError(err, fmt.Errorf("execution of copy command on %s failed: %v", nodeName, err))

	// execute the nexd run script on the test container
	_, err = suite.containerExec(ctx, node, []string{"/bin/bash", "-c", runScript})
	suite.Require().NoError(err)
}

func networkAddr(n *net.IPNet) net.IP {
	network := net.ParseIP("0.0.0.0").To4()
	for i := 0; i < len(n.IP); i++ {
		network[i] = n.IP[i] & n.Mask[i]
	}
	return network
}

// wgDump dump wg sessions for failed test debugging
func (suite *NexodusIntegrationSuite) wgDump(ctx context.Context, container testcontainers.Container) string {
	wgDump, err := suite.containerExec(ctx, container, []string{"wg", "show", "wg0", "dump"})
	if err != nil {
		return ""
	}

	return wgDump
}

// routesDumpV4 dump v4 routes for failed test debugging
func (suite *NexodusIntegrationSuite) routesDumpV4(ctx context.Context, container testcontainers.Container) string {
	routesDump, err := suite.containerExec(ctx, container, []string{"ip", "route"})
	if err != nil {
		return ""
	}

	return routesDump
}

// routesDumpV6 dump v6 routes for failed test debugging
func (suite *NexodusIntegrationSuite) routesDumpV6(ctx context.Context, container testcontainers.Container) string {
	routesDump, err := suite.containerExec(ctx, container, []string{"ip", "-6", "route"})
	if err != nil {
		return ""
	}

	return routesDump
}

// logsDump dump routes for failed test debugging
func (suite *NexodusIntegrationSuite) logsDump(ctx context.Context, container testcontainers.Container) string {
	logsDump, err := suite.containerExec(ctx, container, []string{"cat", "/nexd.logs"})
	if err != nil {
		return "no logs found"
	}

	return logsDump
}

// gatherFail gather details on a failed test for debugging
func (suite *NexodusIntegrationSuite) gatherFail(ctx context.Context, containers ...testcontainers.Container) string {
	var gatherOut []string

	for _, c := range containers {
		ip, _ := getContainerIfaceIP(ctx, inetV4, "wg0", c)
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s wg0 IPv4:\n %s, ", nodeName, ip)
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		ip, _ := getContainerIfaceIP(ctx, inetV6, "wg0", c)
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s wg0 IPv6:\n %s, ", nodeName, ip)
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		ip, _ := getContainerIfaceIP(ctx, inetV4, "eth0", c)
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s eth0 IPv4:\n %s, ", nodeName, ip)
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		ip, _ := getContainerIfaceIP(ctx, inetV6, "eth0", c)
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s eth0 IPv6:\n %s, ", nodeName, ip)
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s wg-dump:\n %s, ", nodeName, suite.wgDump(ctx, c))
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s routes:\n %s, ", nodeName, suite.routesDumpV4(ctx, c))
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		nodeName, _ := c.Name(ctx)
		routes := fmt.Sprintf("%s routes:\n %s, ", nodeName, suite.routesDumpV6(ctx, c))
		gatherOut = append(gatherOut, routes)
	}

	for _, c := range containers {
		nodeName, _ := c.Name(ctx)
		logs := fmt.Sprintf("%s nexd logs:\n %s\n, ", nodeName, suite.logsDump(ctx, c))
		gatherOut = append(gatherOut, logs)
	}

	return strings.Join(gatherOut, "\n")
}

// runCommand runs the cmd and returns the combined stdout and stderr
func (suite *NexodusIntegrationSuite) runCommand(cmd ...string) (string, error) {
	suite.logger.Infof("Running command: %s", strings.Join(cmd, " "))
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %s (%s)", strings.Join(cmd, " "), err, output)
	}

	return string(output), nil
}

// NewTLSConfig creates a *tls.Config configured to trust the .certs/rootCA.pem
func (suite *NexodusIntegrationSuite) NewTLSConfig() *tls.Config {
	dir, err := findCertsDir()
	suite.Require().NoError(err)
	caCert, err := os.ReadFile(filepath.Join(dir, "rootCA.pem"))
	suite.Require().NoError(err)
	caCertPool, err := x509.SystemCertPool()
	suite.Require().NoError(err)
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}
	return tlsConfig
}

// nexdStatus checks for a Running status of the nexd process via nexctl
func (suite *NexodusIntegrationSuite) nexdStatus(ctx context.Context, ctr testcontainers.Container) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*1000)
	defer cancel()
	running, _ := util.CheckPeriodically(timeoutCtx, time.Second, func() (bool, error) {
		statOut, _ := suite.containerExec(ctx, ctr, []string{"/bin/nexctl", "nexd", "status"})
		return strings.Contains(statOut, "Running"), nil
	})
	if running {
		return nil
	}
	nodeName, _ := ctr.Name(ctx)
	return fmt.Errorf("failed to get a 'Running' status from the nexd process in node: %s", nodeName)
}

func (suite *NexodusIntegrationSuite) createNewUser(ctx context.Context, password string) string {
	id, err := suite.createNewUserWithName(ctx, "kitteh", password)
	suite.Require().NoError(err)
	return id
}
func (suite *NexodusIntegrationSuite) createNewUserWithName(ctx context.Context, name string, password string) (string, error) {
	id, err := uuid.NewUUID()
	userName := name + id.String()

	token, err := suite.gocloak.LoginAdmin(suite.Context(), "admin", "floofykittens", "master")
	if err != nil {
		return "", fmt.Errorf("admin login to keycloak failed: %w", err)
	}

	userid, err := suite.gocloak.CreateUser(ctx, token.AccessToken, "nexodus", gocloak.User{
		FirstName: gocloak.StringP("Test"),
		LastName:  gocloak.StringP(name),
		Email:     gocloak.StringP(userName + "@redhat.com"),
		Enabled:   gocloak.BoolP(true),
		Username:  gocloak.StringP(userName),
	})
	if err != nil {
		return "", fmt.Errorf("user create failed: %w", err)
	}

	suite.T().Cleanup(func() {
		token, err := suite.gocloak.LoginAdmin(suite.Context(), "admin", "floofykittens", "master")
		if err != nil {
			_ = suite.gocloak.DeleteUser(ctx, token.AccessToken, "nexodus", userid)
		}
	})

	err = suite.gocloak.SetPassword(ctx, token.AccessToken, userid, "nexodus", password, false)
	if err != nil {
		return "", fmt.Errorf("user set password failed: %w", err)
	}
	return userName, nil
}

func (suite *NexodusIntegrationSuite) getOauth2Token(ctx context.Context, userid, password string) *oauth2.Token {
	jwt, err := suite.gocloak.GetToken(ctx, "nexodus",
		gocloak.TokenOptions{
			ClientID:     gocloak.StringP("nexodus-cli"),
			ClientSecret: gocloak.StringP(""),
			GrantType:    gocloak.StringP("password"),
			Username:     &userid,
			Password:     &password,
			Scope:        gocloak.StringP("openid profile email read:organizations write:organizations read:users write:users read:devices write:devices"),
		})
	suite.Require().NoError(err)
	return &oauth2.Token{
		AccessToken:  jwt.AccessToken,
		TokenType:    jwt.TokenType,
		RefreshToken: jwt.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(jwt.ExpiresIn) * time.Second),
	}
}

// getNodeHostname trims the container ID down to the node hostname
func (suite *NexodusIntegrationSuite) getNodeHostname(ctx context.Context, ctr testcontainers.Container) (string, error) {
	var hostname string
	err := backoff.Retry(func() error {
		cid := ctr.GetContainerID()
		if len(cid) == 12 {
			hostname = ctr.GetContainerID()
		}
		if len(cid) < 12 {
			return fmt.Errorf("invalid container ID: %s", ctr.GetContainerID())
		} else {
			hostname = strings.TrimSpace(cid[:12])
		}
		return nil
	}, backoff.WithContext(backoff.NewConstantBackOff(1*time.Second), ctx))

	return hostname, err
}
