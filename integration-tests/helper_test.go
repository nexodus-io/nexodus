package integration_tests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/database"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/nexodus-io/nexodus/internal/models"

	"github.com/ahmetb/dlog"
	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/nexodus"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"go.uber.org/zap/zaptest"
)

const (
	protoTCP = "tcp"
	protoUDP = "udp"
)

type Helper struct {
	*testing.T
	require *require.Assertions
	assert  *assert.Assertions
}

func NewHelper(t *testing.T) *Helper {
	return &Helper{
		T:       t,
		require: require.New(t),
		assert:  assert.New(t),
	}
}

func callerSourceLine() string {
	if _, helperFile, _, ok := runtime.Caller(0); ok {
		for skip := 1; skip < 20; skip++ {
			if _, file, line, ok := runtime.Caller(skip); ok {
				if file != helperFile {
					return fmt.Sprintf("%s:%d:", filepath.Base(file), line)
				}
			} else {
				break
			}
		}
	}
	return ""
}

func (helper *Helper) log(args ...any) {
	caller := callerSourceLine()
	if caller != "" {
		args = append([]any{caller}, args...)
	}
	helper.T.Log(args...)
}
func (helper *Helper) logf(fmt string, args ...any) {
	caller := callerSourceLine()
	if caller != "" {
		args = append([]any{caller}, args...)
		helper.T.Logf("%s "+fmt, args...)
	} else {
		helper.T.Logf(fmt, args...)
	}
}

// CreateNode creates a container
func (helper *Helper) CreateNode(ctx context.Context, nameSuffix string, networks []string, v6 v6Enable) (testcontainers.Container, func()) {

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

// containerExec exec container commands
func (helper *Helper) containerExec(ctx context.Context, container testcontainers.Container, cmd []string) (string, error) {
	nodeName, _ := container.Name(ctx)
	if cmd[0] != "cat" && cmd[0] != "nft" {
		helper.logf("Running command on %s: %s", nodeName, strings.Join(cmd, " "))
	}
	code, reader, err := container.Exec(
		ctx,
		cmd,
	)
	if err != nil {
		return "", err
	}

	reader = dlog.NewReader(reader)
	output, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("exit code %d. stderr: %s", code, string(output))
	}

	return string(output), err
}

// CreateNetwork creates a docker network
func (helper *Helper) CreateNetwork(ctx context.Context, name, cidr string) testcontainers.Network {
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
	require.NoError(helper.T, err)
	return net
}

func (helper *Helper) runNexd(ctx context.Context, node testcontainers.Container, args ...string) {
	helper.runNexdWithEnv(ctx, node, nil, args...)
}

// runNexd copies the nexd command to a file on the container and runs it piping the logs to a file
func (helper *Helper) runNexdWithEnv(ctx context.Context, node testcontainers.Container, envs []string, args ...string) {
	nodeName, _ := node.Name(ctx)
	runScript := fmt.Sprintf("%s-nexd-run.sh", strings.TrimPrefix(nodeName, "/"))
	runScriptLocal := fmt.Sprintf("tmp/%s", runScript)
	cmd := []string{"NEXD_LOGLEVEL=debug"}
	cmd = append(cmd, envs...)
	cmd = append(cmd, "/bin/nexd")
	cmd = append(cmd, "--stun-server", fmt.Sprintf("%s:%d", hostDNSName, testStunServer1Port))
	cmd = append(cmd, "--stun-server", fmt.Sprintf("%s:%d", hostDNSName, testStunServer2Port))
	cmd = append(cmd, "--service-url", "https://try.nexodus.127.0.0.1.nip.io")
	cmd = append(cmd, args...)
	cmd = append(cmd, ">> /nexd.logs 2>&1 &")

	// write the nexd run command to a local file
	logger := zaptest.NewLogger(helper.T).Sugar()
	nexodus.WriteToFile(logger, strings.Join(cmd, " "), runScriptLocal, 0755)
	// copy the nexd run script to the test container
	err := node.CopyFileToContainer(ctx, runScriptLocal, fmt.Sprintf("/bin/%s", runScript), 0755)
	helper.require.NoError(err, fmt.Errorf("execution of copy command on %s failed: %w", nodeName, err))

	// execute the nexd run script on the test container
	helper.logf("Running command on %s: %s", nodeName, strings.Join(cmd, " "))
	_, err = helper.containerExec(ctx, node, []string{"/bin/bash", "-c", runScript})
	helper.require.NoError(err)
}

// routesDumpV4 dump v4 routes for failed test debugging
func (helper *Helper) routesDumpV4(ctx context.Context, container testcontainers.Container) string {
	routesDump, err := helper.containerExec(ctx, container, []string{"ip", "route"})
	if err != nil {
		return ""
	}

	return routesDump
}

// routesDumpV6 dump v6 routes for failed test debugging
func (helper *Helper) routesDumpV6(ctx context.Context, container testcontainers.Container) string {
	routesDump, err := helper.containerExec(ctx, container, []string{"ip", "-6", "route"})
	if err != nil {
		return ""
	}

	return routesDump
}

// logsDump dump routes for failed test debugging
func (helper *Helper) logsDump(ctx context.Context, container testcontainers.Container) string {
	logsDump, err := helper.containerExec(ctx, container, []string{"cat", "/nexd.logs"})
	if err != nil {
		return "no logs found"
	}

	return logsDump
}

// gatherFail gather details on a failed test for debugging
func (helper *Helper) gatherFail(c testcontainers.Container) string {
	var res []string

	ctx := context.Background()
	nodeName, _ := c.Name(ctx)

	ip, err := getContainerIfaceIPNoRetry(ctx, inetV4, "wg0", c)
	if err != nil {
		helper.log(err)
	}
	res = append(res, fmt.Sprintf("%s wg0 IPv4: %s", nodeName, ip))
	ip, err = getContainerIfaceIPNoRetry(ctx, inetV6, "wg0", c)
	if err != nil {
		helper.log(err)
	}
	res = append(res, fmt.Sprintf("%s wg0 IPv6: %s", nodeName, ip))
	ip, err = getContainerIfaceIPNoRetry(ctx, inetV4, "eth0", c)
	if err != nil {
		helper.log(err)
	}
	res = append(res, fmt.Sprintf("%s eth0 IPv4: %s", nodeName, ip))
	ip, err = getContainerIfaceIPNoRetry(ctx, inetV6, "eth0", c)
	if err != nil {
		helper.log(err)
	}
	res = append(res, fmt.Sprintf("%s eth0 IPv6: %s", nodeName, ip))
	res = append(res, fmt.Sprintf("%s routes IPv4:\n%s, ", nodeName, helper.routesDumpV4(ctx, c)))
	res = append(res, fmt.Sprintf("%s routes IPv6:\n%s, ", nodeName, helper.routesDumpV6(ctx, c)))
	res = append(res, fmt.Sprintf("%s nexd logs:\n%s\n, ", nodeName, helper.logsDump(ctx, c)))

	return strings.Join(res, "\n")
}

// runCommand runs the cmd and returns the combined stdout and stderr
func (helper *Helper) runCommand(cmd ...string) (string, error) {
	helper.logf("Running command: %s", strings.Join(cmd, " "))
	// #nosec G204
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run %q: %w (%s)", strings.Join(cmd, " "), err, output)
	}

	return string(output), nil
}

func (helper *Helper) MustRunJsonCommand(result interface{}, cmd ...string) {
	helper.logf("Running command: %s", strings.Join(cmd, " "))
	// #nosec G204
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	helper.require.NoErrorf(err, "command: %q: failed with: %v (%s)", strings.Join(cmd, " "), err, output)
	err = json.Unmarshal(output, &result)
	helper.require.NoErrorf(err, "command: %q: unmarshal errror: %v (%s)", strings.Join(cmd, " "), err, output)
}

type SecretsList struct {
	helper *Helper
	Items  []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Data map[string]string `json:"data"`
	} `json:"items"`
}

func (sl *SecretsList) Get(namePrefix string, key string) string {
	for _, item := range sl.Items {
		if strings.HasPrefix(item.Metadata.Name, namePrefix) {
			v, err := base64.StdEncoding.DecodeString(item.Data[key])
			sl.helper.require.NoError(err)
			return string(v)
		}
	}
	return ""
}

func (helper *Helper) GetAllKubeSecrets() SecretsList {
	result := SecretsList{}
	helper.MustRunJsonCommand(&result, "kubectl", "-n", "nexodus", "get", "secrets", "-o", "json")
	result.helper = helper
	return result
}

func (helper *Helper) StartPortForwardToDB(ctx context.Context) (db *gorm.DB, close func()) {
	ctx, cancel := context.WithCancel(ctx)
	secrets := helper.GetAllKubeSecrets()
	dbUser := secrets.Get("database-pguser-apiserver", "user")
	dbPassword := secrets.Get("database-pguser-apiserver", "password")
	dbHost := secrets.Get("database-pguser-apiserver", "pgbouncer-host")
	dbPort := secrets.Get("database-pguser-apiserver", "pgbouncer-port")

	// find a free port to use...
	l, err := net.Listen("tcp", "127.0.0.1:0")
	helper.require.NoError(err)
	_, localPort, err := net.SplitHostPort(l.Addr().String())
	helper.require.NoError(err)
	_ = l.Close()

	service := fmt.Sprintf("service/%s", strings.TrimSuffix(dbHost, ".nexodus.svc"))
	portMapping := fmt.Sprintf("%s:%s", localPort, dbPort)
	cmd := exec.CommandContext(ctx, "kubectl", "-n", "nexodus", "port-forward", service, portMapping)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	helper.require.NoError(err)
	close = func() {
		cancel() // to kill the process
		_ = cmd.Wait()
	}
	defer func() {
		// if we fail to create a connection to the DB...
		// stop the port forward
		if db == nil {
			close()
		}
	}()

	// wait for the port forward to be up and running
	helper.require.Eventually(func() bool {
		c, err := net.Dial("tcp", "localhost:"+localPort)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}, time.Second*10, time.Millisecond*100, "port forward is not up and running yet")

	gormConfig := &gorm.Config{
		Logger: database.NewLogger(zap.NewNop().Sugar()),
	}

	// try to connect using TLS...
	db, err = gorm.Open(postgres.Open(
		fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			"localhost", dbUser, dbPassword, "apiserver", localPort, "require"),
	), gormConfig)
	if err != nil && strings.Contains(err.Error(), "tls error") {
		// fallback to non-TLS
		db, err = gorm.Open(postgres.Open(
			fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
				"localhost", dbUser, dbPassword, "apiserver", localPort, "disable"),
		), gormConfig)
	}
	helper.require.NoError(err)

	res := db.Exec("SELECT 1 FROM users")
	helper.require.NoError(res.Error)

	return
}
func (helper *Helper) MustRunCommand(cmd ...string) string {
	helper.logf("Running command: %s", strings.Join(cmd, " "))
	// #nosec G204
	output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
	helper.require.NoError(err, fmt.Errorf("command: %q: failed with: %w (%s)", strings.Join(cmd, " "), err, output))
	return string(output)
}

// nexdStopped checks to see if nexd is stopped. It assumes if we get an error trying to talk to it with nexctl
// that it must be stopped.
func (helper *Helper) nexdStopped(ctx context.Context, ctr testcontainers.Container) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	stopped, _ := util.CheckPeriodically(timeoutCtx, time.Second, func() (bool, error) {
		statOut, err := helper.containerExec(ctx, ctr, []string{"/bin/nexctl", "nexd", "status"})
		helper.Logf("nexd status: %s", statOut)
		return err != nil, nil
	})
	if !stopped {
		nodeName, _ := ctr.Name(ctx)
		return fmt.Errorf("failed to validate the nexd process has stopped in node: %s", nodeName)
	}
	return nil
}

// nexdStatus checks for a Running status of the nexd process via nexctl
func (helper *Helper) nexdStatus(ctx context.Context, ctr testcontainers.Container) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()
	running, _ := util.CheckPeriodically(timeoutCtx, time.Second, func() (bool, error) {
		statOut, _ := helper.containerExec(ctx, ctr, []string{"/bin/nexctl", "nexd", "status"})
		helper.Logf("nexd status: %s", statOut)
		return strings.Contains(statOut, "Running"), nil
	})
	if !running {
		nodeName, _ := ctr.Name(ctx)
		return fmt.Errorf("failed to get a 'Running' status from the nexd process in node: %s", nodeName)
	}

	// This really should not be necessary. If we had a better state machine for nexd, we could
	// know that nexd is running and the data plane is up. Right now, READY just means nexd has
	// successfully connected with the control plane. Attempts to get data plane info may happen
	// too soon. This is a hack to make sure nexd has had enough time to get its own local config.
	// Related: https://github.com/nexodus-io/nexodus/pull/886
	timeoutCtx2, cancel2 := context.WithTimeout(ctx, time.Second*60)
	defer cancel2()
	gotIps, _ := util.CheckPeriodically(timeoutCtx2, time.Second, func() (bool, error) {
		tunIPv4, _ := getTunnelIP(ctx, helper, inetV4, ctr)
		helper.Logf("nexd tunnelIP: %s", tunIPv4)
		if tunIPv4 == "" {
			return false, nil
		}

		tunIPv6, _ := getTunnelIP(ctx, helper, inetV6, ctr)
		helper.Logf("nexd tunnelIP v6: %s", tunIPv6)
		if tunIPv6 == "" {
			return false, nil
		}

		return true, nil
	})
	if !gotIps {
		nodeName, _ := ctr.Name(ctx)
		return fmt.Errorf("failed to get tunnel IPs from the nexd process in node: %s", nodeName)
	}

	return nil
}

func (helper *Helper) createNewUser(ctx context.Context, password string) (string, func()) {
	id, cleanup, err := createNewUserWithName(ctx, "kitteh", password)
	helper.require.NoError(err)
	return id, cleanup
}

// getNodeHostname trims the container ID down to the node hostname
func (helper *Helper) getNodeHostname(ctx context.Context, ctr testcontainers.Container) (string, error) {
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

// createSecurityRule creates a rule to append to security group rules
func (helper *Helper) createSecurityRule(protocol string, fromPortStr, toPortStr string, ipRanges []string) public.ModelsSecurityRule {
	fromPort := int32(0)
	toPort := int32(0)

	// Check and convert fromPort from string to int32 if not empty
	if fromPortStr != "" {
		tmpFromPort, err := strconv.ParseInt(fromPortStr, 10, 32)
		if err != nil {
			helper.Errorf("failed to parse a valid security rule port from %s: %v", fromPortStr, err)
		}
		fromPort = int32(tmpFromPort)
	}

	// Check and convert toPort from string to int32 if not empty
	if toPortStr != "" {
		tmpToPort, err := strconv.ParseInt(toPortStr, 10, 32)
		if err != nil {
			helper.Errorf("failed to parse a valid security rule port from %s: %v", fromPortStr, err)
		}
		toPort = int32(tmpToPort)
	}

	// If ipRanges is nil, initialize it as an empty slice
	if ipRanges == nil {
		ipRanges = []string{}
	}

	// Create the rule
	rule := public.ModelsSecurityRule{
		IpProtocol: protocol,
		FromPort:   fromPort,
		ToPort:     toPort,
		IpRanges:   ipRanges,
	}

	return rule
}

// retryNftCmdOnAllNodes is a wrapper for retryNftCmdUntilNotEqual that takes the nftables from before a
// security group is updated and diffs them for all the nodes passed until it detects a change or times out.
func (helper *Helper) retryNftCmdOnAllNodes(ctx context.Context, nodes []testcontainers.Container, command []string, cmdOutputBefore []string) (bool, error) {
	helper.Logf("waiting for the security group change to converge")
	for i, node := range nodes {
		success, err := helper.retryNftCmdUntilNotEqual(ctx, node, command, cmdOutputBefore[i])
		if err != nil {
			return false, err
		}
		if !success {
			return false, fmt.Errorf("output did not change for one of the nodes after max attempts")
		}
	}
	return true, nil
}

// retryNftCmdUntilNotEqual is used to watch and wait for the new policy to be applied to the device
func (helper *Helper) retryNftCmdUntilNotEqual(ctx context.Context, ctr testcontainers.Container, command []string, cmdOutputBefore string) (bool, error) {
	const maxRetries = 30             // as the polling timer gets lowered, so can this value
	const retryInterval = time.Second // Retry every second

	for i := 0; i < maxRetries; i++ {
		cmdOutputAfter, err := helper.containerExec(ctx, ctr, command)
		if err != nil {
			return false, err
		}

		// Filter out state tracking line that is always present and strip counters
		linesBefore := strings.Split(cmdOutputBefore, "\n")
		linesAfter := strings.Split(cmdOutputAfter, "\n")
		filteredBefore := filterAndTrimLines(linesBefore, "established", "counter")
		filteredAfter := filterAndTrimLines(linesAfter, "established", "counter")

		if filteredBefore != filteredAfter {
			// They are not equal, meaning the nftable has changed
			return true, nil
		}

		// They are equal, meaning the nftable has not changed
		time.Sleep(retryInterval)
	}

	// They are still equal after maxRetries attempts
	return false, fmt.Errorf("output did not change after %d attempts", maxRetries)
}

// startPortListener using netcat, start a listener that will respond with the device's
// hostname when anything connects to the listening port
func (helper *Helper) startPortListener(ctx context.Context, ctr testcontainers.Container, ipAddr, proto, port string) error {
	nodeHostname, err := helper.getNodeHostname(ctx, ctr)
	if err != nil {
		return fmt.Errorf("failed to get the device's hostname for device")
	}
	var protoOpt string
	if proto == protoTCP {
		protoOpt = "-t"
	} else {
		protoOpt = "-u"
	}
	cmd := []string{
		"bash",
		"-c",
		fmt.Sprintf("while true; do echo %s | nc -l %s %s %s; done &", nodeHostname, ipAddr, port, protoOpt),
	}

	_, err = helper.containerExec(ctx, ctr, cmd)

	return err
}

// connectToPort using netcat, connect to a listener and return the output from the connection
func (helper *Helper) connectToPort(ctx context.Context, ctr testcontainers.Container, ipv4, proto, port string) (string, error) {
	var protoOpt string
	if proto == protoTCP {
		protoOpt = "-t"
	} else {
		protoOpt = "-u"
	}
	command := []string{"bash",
		"-c",
		fmt.Sprintf("echo -e '\\n' | nc -w 2 %s %s %s", ipv4, port, protoOpt),
	}

	netcatOut, err := helper.containerExec(ctx, ctr, command)

	return strings.TrimSuffix(netcatOut, "\n"), err
}

// securityGroupRulesUpdate update security group rule
func (helper *Helper) securityGroupRulesUpdate(username, password string, inboundRules []public.ModelsSecurityRule, outboundRules []public.ModelsSecurityRule, secGroupID string) error {
	// Marshal rules to JSON
	inboundJSON, err := json.Marshal(inboundRules)
	if err != nil {
		return err
	}

	outboundJSON, err := json.Marshal(outboundRules)
	if err != nil {
		return err
	}

	command := []string{
		nexctl,
		"--username", username,
		"--password", password,
		"security-group", "update",
		"--security-group-id", secGroupID,
		"--inbound-rules", string(inboundJSON),
		"--outbound-rules", string(outboundJSON),
	}

	out, err := helper.runCommand(command...)
	if err != nil {
		return err
	}

	helper.Logf("nexctl security-group update output:\n%s", out)

	return nil
}

func (helper *Helper) createVPC(username string, password string, args ...string) string {
	orgOut, err := helper.runCommand(append([]string{
		nexctl,
		"--username", username, "--password", password,
		"--output", "json",
		"vpc", "create",
		"--description", "Test: " + helper.T.Name(),
	}, args...)...)
	helper.require.NoError(err)
	var org models.VPC
	err = json.Unmarshal([]byte(orgOut), &org)
	helper.require.NoError(err)
	return org.ID.String()
}

func (helper *Helper) deleteVPC(username string, password string, orgID string) error {
	_, err := helper.runCommand(nexctl,
		"--username", username, "--password", password,
		"vpc", "delete", "--vpc-id", orgID)
	return err
}
func (helper *Helper) peerListNexdDevices(ctx context.Context, ctr testcontainers.Container) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()
	running, _ := util.CheckPeriodically(timeoutCtx, time.Second, func() (bool, error) {
		statOut, err := helper.containerExec(ctx, ctr, []string{"/bin/nexctl", "nexd", "peers", "list"})
		helper.Logf("nexd device peer list: %s", statOut)
		return err != nil, err
	})
	if !running {
		nodeName, _ := ctr.Name(ctx)
		return fmt.Errorf("failed to run nexd device peer list in node: %s", nodeName)
	}
	return nil
}
