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
	"golang.org/x/oauth2"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Nerzal/gocloak/v13"
	"github.com/cenkalti/backoff/v4"
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
func (suite *NexodusIntegrationSuite) CreateNode(ctx context.Context, name string, networks []string) testcontainers.Container {

	certsDir, err := findCertsDir()
	require.NoError(suite.T(), err)

	req := testcontainers.ContainerRequest{
		Image:    "quay.io/nexodus/test:ubuntu",
		Name:     name,
		Networks: networks,
		CapAdd: []string{
			"SYS_MODULE",
			"NET_ADMIN",
			"NET_RAW",
		},
		ExtraHosts: []string{
			fmt.Sprintf("try.nexodus.local:%s", hostDNSName),
			fmt.Sprintf("api.try.nexodus.local:%s", hostDNSName),
			fmt.Sprintf("auth.try.nexodus.local:%s", hostDNSName),
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

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, "/", "-")
}
func newClient(ctx context.Context, username, password string) (*client.Client, error) {
	return client.NewClient(ctx, "http://api.try.nexodus.local", nil, client.WithPasswordGrant(username, password))
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
func (suite *NexodusIntegrationSuite) containerExec(ctx context.Context, container testcontainers.Container, cmd []string) (string, error) {
	code, outputRaw, err := container.Exec(
		ctx,
		cmd,
	)
	if err != nil {
		return "", err
	}
	nodeName, _ := container.Name(ctx)
	if cmd[0] != "/bin/nexd" {
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

func (suite *NexodusIntegrationSuite) runNexd(ctx context.Context, node testcontainers.Container, args ...string) {
	nodeName, _ := node.Name(ctx)
	runScript := fmt.Sprintf("%s-nexd-run.sh", strings.TrimPrefix(nodeName, "/"))
	runScriptLocal := fmt.Sprintf("tmp/%s", runScript)
	cmd := []string{"/bin/nexd"}
	cmd = append(cmd, args...)
	cmd = append(cmd, "https://try.nexodus.local")
	cmd = append(cmd, ">> /nexd.logs 2>&1 &")

	// write the nexd run command to a local file
	nexodus.WriteToFile(suite.logger, strings.Join(cmd, " "), runScriptLocal, 0755)
	// copy the nexd run script to the test container
	err := node.CopyFileToContainer(ctx, runScriptLocal, fmt.Sprintf("/bin/%s", runScript), 0755)
	if suite.T().Failed() {
		suite.logger.Errorf("execution of copy command on %s failed: %v", nodeName, err)
	}
	// execute the nexd run script on the test container
	out, err := suite.containerExec(ctx, node, []string{"/bin/bash", "-c", runScript})
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
func (suite *NexodusIntegrationSuite) wgDump(ctx context.Context, container testcontainers.Container) string {
	wgDump, err := suite.containerExec(ctx, container, []string{"wg", "show", "wg0", "dump"})
	if err != nil {
		return ""
	}

	return wgDump
}

// routesDump dump routes for failed test debugging
func (suite *NexodusIntegrationSuite) routesDump(ctx context.Context, container testcontainers.Container) string {
	routesDump, err := suite.containerExec(ctx, container, []string{"ip", "route"})
	if err != nil {
		return ""
	}

	return routesDump
}

// routesDump dump routes for failed test debugging
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
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*10)
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
	id, err := uuid.NewUUID()
	suite.Require().NoError(err)
	userName := "kitteh-" + id.String()

	token, err := suite.gocloak.LoginAdmin(suite.Context(), "admin", "floofykittens", "master")
	suite.Require().NoError(err)

	userid, err := suite.gocloak.CreateUser(ctx, token.AccessToken, "nexodus", gocloak.User{
		FirstName: gocloak.StringP("Test"),
		LastName:  gocloak.StringP(userName),
		Email:     gocloak.StringP(userName + "@example.com"),
		Enabled:   gocloak.BoolP(true),
		Username:  gocloak.StringP(userName),
	})
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		token, err := suite.gocloak.LoginAdmin(suite.Context(), "admin", "floofykittens", "master")
		if err != nil {
			_ = suite.gocloak.DeleteUser(ctx, token.AccessToken, "nexodus", userid)
		}
	})

	err = suite.gocloak.SetPassword(ctx, token.AccessToken, userid, "nexodus", password, false)
	suite.Require().NoError(err)
	return userName
}

func (suite *NexodusIntegrationSuite) getOauth2Token(ctx context.Context, userid, password string) *oauth2.Token {
	jwt, err := suite.gocloak.Login(ctx, "nexodus-cli", "", "nexodus", userid, password)
	suite.Require().NoError(err)
	return &oauth2.Token{
		AccessToken:  jwt.AccessToken,
		TokenType:    jwt.TokenType,
		RefreshToken: jwt.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(jwt.ExpiresIn) * time.Second),
	}
}
