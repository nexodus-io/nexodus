//go:build integration

package integration_tests

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/state"
	"github.com/testcontainers/testcontainers-go"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
)

// TestProxyEgress tests that nexd proxy can be used with a single egress rule
func TestProxyEgress(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy", "--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")))
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node1
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node1, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node1, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node2 (to the local proxy) to reach the server on node1
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://127.0.0.1"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	_, _ = helper.containerExec(ctx, node1, []string{"killall", "python3"})
	wg.Wait()
}

// TestProxyEgressUDP tests that nexd proxy can be used with a single UDP egress rule
func TestProxyEgressUDP(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	helper.Logf("Starting nexd on node1")
	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getTunnelIP(ctx, helper, inetV4, node1)
	require.NoError(err)
	node1IPv6, err := getTunnelIP(ctx, helper, inetV6, node1)
	require.NoError(err)

	helper.Logf("Starting nexd on node2")
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--egress", fmt.Sprintf("udp:4242:%s", net.JoinHostPort(node1IP, "4242")),
		"--egress", fmt.Sprintf("udp:4243:%s", net.JoinHostPort(node1IPv6, "4242")))
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an UDP server on node1
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, _ = helper.containerExec(ctx, node1, []string{"udpong", "4242"})
	})

	// run a UDP client on node2 (to the local proxy) to reach the server on node1
	ctxTimeout, clientCancel := context.WithTimeout(ctx, 10*time.Second)
	defer clientCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		targets := []struct {
			IP   string
			Port string
		}{
			// v4 client, v4 server
			{IP: "127.0.0.1", Port: "4242"},
			// v4 client, v6 server
			{IP: "127.0.0.1", Port: "4243"},
			// v6 client, v4 server
			{IP: "::1", Port: "4242"},
			// v6 client, v6 server
			{IP: "::1", Port: "4243"},
		}
		for _, t := range targets {
			output, err := helper.containerExec(ctx, node2, []string{"udping", t.IP, t.Port})
			if err != nil {
				helper.Logf("Retrying udp client for up to 10 seconds: %v -- %s", err, output)
				return false, nil
			}
			require.True(strings.Contains(output, "pong"))
		}
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	_, _ = helper.containerExec(ctx, node1, []string{"killall", "udpong"})
	wg.Wait()
}

// TestProxyEgress tests that nexd proxy can be used with multiple egress rules
func TestProxyEgressMultipleRules(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getTunnelIP(ctx, helper, inetV4, node1)
	require.NoError(err)
	node1IPv6, err := getTunnelIP(ctx, helper, inetV6, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")),
		"--egress", fmt.Sprintf("tcp:81:%s", net.JoinHostPort(node1IPv6, "8080")))
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node1
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node1, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node1, []string{"python3", "-m", "http.server", "-b", "::", "8080"})
	})

	// run curl on node2 (to the local proxy) to reach the server on node1
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		targets := []struct {
			IP   string
			Port string
		}{
			// v4 client, v4 server
			{IP: "127.0.0.1", Port: "80"},
			// v4 client, v6 server
			{IP: "127.0.0.1", Port: "81"},
			// v6 client, v4 server
			{IP: "::1", Port: "80"},
			// v6 client, v6 server
			{IP: "::1", Port: "81"},
		}
		for _, target := range targets {
			output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(target.IP, target.Port))})
			if err != nil {
				helper.Logf("Retrying curl for up to 10 seconds: %v -- %s", err, output)
				return false, nil
			}
			require.True(strings.Contains(output, "bananas"))
		}
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	_, _ = helper.containerExec(ctx, node1, []string{"killall", "python3"})
	wg.Wait()
}

// TestProxyIngress tests that nexd proxy with a single ingress rule
func TestProxyIngress(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy", "--ingress", "tcp:8080:127.0.0.1:8080")
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node2
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node2, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node2, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node1 to the server on node2 (running the proxy)
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(node2IP, "8080"))})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	_, _ = helper.containerExec(ctx, node2, []string{"killall", "python3"})
	wg.Wait()
}

// TestProxyIngressUDP tests that nexd proxy can be used with a single UDP ingress rule
func TestProxyIngressUDP(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()
	node2, stop := helper.CreateNode(ctx, "node2", []string{defaultNetwork}, enableV6)
	defer stop()

	helper.Logf("Starting nexd on node1")
	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.Logf("Starting nexd on node2")
	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--ingress", "udp:4242:127.0.0.1:4242",
		"--ingress", "udp:4243:[::1]:4242")
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)
	node2IPv6, err := getTunnelIP(ctx, helper, inetV6, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an UDP server on node2
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, _ = helper.containerExec(ctx, node2, []string{"udpong", "4242"})
	})

	// run a UDP client on node1 to reach the remote udp proxy on node1
	ctxTimeout, clientCancel := context.WithTimeout(ctx, 10*time.Second)
	defer clientCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		targets := []struct {
			IP   string
			Port string
		}{
			// v4 client, v4 server
			{IP: node2IP, Port: "4242"},
			// v6 client, v4 server
			{IP: node2IPv6, Port: "4242"},
			// v4 client, v6 server
			{IP: node2IP, Port: "4243"},
			// v6 client, v6 server
			{IP: node2IPv6, Port: "4243"},
		}
		for _, t := range targets {
			output, err := helper.containerExec(ctx, node1, []string{"udping", t.IP, t.Port})
			if err != nil {
				helper.Logf("Retrying udp client for up to 10 seconds: %v -- %s", err, output)
				return false, nil
			}
			require.True(strings.Contains(output, "pong"))
		}
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	_, _ = helper.containerExec(ctx, node2, []string{"killall", "udpong"})
	wg.Wait()
}

// TestProxyIngress tests that nexd proxy with multiple ingress rules
func TestProxyIngressMultipleRules(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")),
		"--ingress", fmt.Sprintf("tcp:8081:%s", net.JoinHostPort("::1", "8080")))
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)
	node2IPv6, err := getTunnelIP(ctx, helper, inetV6, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node2
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node2, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node2, []string{"python3", "-m", "http.server", "-b", "::", "8080"})
	})

	// run curl on node1 to the server on node2 (running the proxy)
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		targets := []struct {
			IP   string
			Port string
		}{
			// v4 client, v4 server
			{IP: node2IP, Port: "8080"},
			// v4 client, v6 server
			{IP: node2IP, Port: "8081"},
			// v6 client, v4 server
			{IP: node2IPv6, Port: "8080"},
			// v6 client, v6 server
			{IP: node2IPv6, Port: "8081"},
		}
		for _, target := range targets {
			output, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(target.IP, target.Port))})
			if err != nil {
				helper.Logf("Retrying curl for up to 10 seconds: %v -- %s", err, output)
				return false, nil
			}
			require.True(strings.Contains(output, "bananas"))
		}
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	_, _ = helper.containerExec(ctx, node2, []string{"killall", "python3"})
	wg.Wait()
}

// TestProxyIngressAndEgress tests that a proxy can be used to both ingress and egress traffic
func TestProxyIngressAndEgress(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getTunnelIP(ctx, helper, inetV4, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")),
		"--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")))
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// run an http server on node1 and node2
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node2, []string{"python3", "-c", "import os; open('index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node2, []string{"python3", "-m", "http.server", "8080"})
	})
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node1, []string{"python3", "-c", "import os; open('index.html', 'w').write('pancakes')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node1, []string{"python3", "-m", "http.server", "8080"})
	})

	// run curl on node1 to the server on node2 (this exercises the egress rule)
	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(node2IP, "8080"))})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)

	// run curl on node2 (to the local proxy) to reach the server on node1 (this exercises the ingress rule)
	ctxTimeout, curlCancel = context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()
	success, err = util.CheckPeriodically(ctxTimeout, time.Second, func() (bool, error) {
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://127.0.0.1"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "pancakes"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	_, _ = helper.containerExec(ctx, node1, []string{"killall", "python3"})
	_, _ = helper.containerExec(ctx, node2, []string{"killall", "python3"})
	wg.Wait()
}

// Test invalid proxy configuration
func TestProxyInvalidConfig(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()

	baseArgs := []string{"nexd", "--username", username, "--password", password, "proxy"}
	proxyArgs := [][]string{
		// duplicate tcp ingress port
		{"--ingress", "tcp:8080:127.0.0.1:80", "--ingress", "tcp:8080:127.0.0.2:81"},
		// duplicate tcp egress port
		{"--egress", "tcp:8080:100.100.0.1:80", "--egress", "tcp:8080:100.100.0.2:81"},
		// duplicate udp ingress port
		{"--ingress", "udp:4242:127.0.0.1:4242", "--ingress", "udp:4242:127.0.0.2:4243"},
		// duplicate udp egress port
		{"--egress", "udp:4242:100.100.0.1:4242", "--egress", "udp:4242:100.100.0.2:4243"},
		// Invalid ingress tcp listen port
		{"--ingress", "tcp:90000:127.0.0.1:8080"},
		// Invalid egress tcp listen port
		{"--egress", "tcp:90000:100.100.0.1:8080"},
		// Invalid ingress udp listen port
		{"--ingress", "udp:90000:127.0.0.1:4242"},
		// Invalid egress udp listen port
		{"--egress", "udp:0:100.10j0.0.1:4242"},
		// Invalid ingress tcp connect port
		{"--ingress", "tcp:8080:127.0.0.1:90000"},
		// Invalid egress tcp connect port
		{"--egress", "tcp:8080:100.100.0.1:0"},
		// Invalid ingress udp connect port
		{"--ingress", "udp:4242:127.0.0.1:90000"},
		// Invalid egress udp connect port
		{"--egress", "udp:4242:100.100.0.1:0"},
		// Invalid ingress protocol
		{"--ingress", "tcpa:8080:127.0.0.1:80"},
		// Invalid egress protocol
		{"--egress", "tcpa:8080:100.100.0.1:80"},
		// Incomplete ingress rule
		{"--ingress", "tcp:8080"},
		// Incomplete egress rule
		{"--egress", "tcp:8080"},
		// destination host can not be blank
		{"--egress", "tcp:8080::80"},
	}

	for _, args := range proxyArgs {
		out, err := helper.containerExec(ctx, node1, append(baseArgs, args...))
		if err == nil {
			if strings.Contains(out, "level\":\"fatal") {
				// containerExec() should have returned non-zero, but sometimes it doesn't ...
				err = fmt.Errorf("fatal error in nexd output: %s", out)
			} else {
				// This test will fail. Print the output just in case there's a hint in there
				// about what went wrong.
				helper.Logf("nexd output: %s", out)
			}
		}
		require.Error(err)
	}
}

func TestProxyNexctl(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	password := "floofykittens"
	username, cleanup := helper.createNewUser(ctx, password)
	defer cleanup()

	// create the nodes
	node1, stop := helper.CreateNode(ctx, "node1", []string{defaultNetwork}, enableV6)
	defer stop()

	// start nexodus on the nodes
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "proxy")

	// validate nexd has started
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	// No rules yet
	out, err := helper.containerExec(ctx, node1, []string{"nexctl", "nexd", "proxy", "list"})
	require.NoError(err)
	require.Equal(out, "")

	// Connecting to port 80 should fail, nothing is listening
	_, err = helper.containerExec(ctx, node1, []string{"curl", "http://127.0.0.1"})
	require.Error(err)

	// Start a listener on port 8080 that we'll hit with a loopback through the proxy
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err := helper.containerExec(ctx, node1, []string{"python3", "-c", "import os; open('index.html', 'w').write('waffles')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, node1, []string{"python3", "-m", "http.server", "8080"})
	})

	node1IP, err := getTunnelIP(ctx, helper, inetV4, node1)
	require.NoError(err)

	// Dynamically add a set of loopback proxy rules
	_, err = helper.containerExec(ctx, node1, []string{"nexctl", "nexd", "proxy", "add",
		"--ingress", "tcp:4242:127.0.0.1:8080", "--egress", fmt.Sprintf("tcp:80:%s:4242", node1IP)})
	require.NoError(err)

	// Rules are stored
	data, err := helper.containerExec(ctx, node1, []string{"cat", "/var/lib/nexd/state.json"})
	require.NoError(err)
	s := state.State{}
	err = json.Unmarshal([]byte(data), &s)
	require.NoError(err)
	require.Equal(state.ProxyRulesConfig{
		Egress:  []string{fmt.Sprintf("tcp:80:%s:4242", node1IP)},
		Ingress: []string{"tcp:4242:127.0.0.1:8080"},
	}, s.ProxyRulesConfig)

	// Rules are present now
	out, err = helper.containerExec(ctx, node1, []string{"nexctl", "nexd", "proxy", "list"})
	require.NoError(err)
	require.True(strings.Contains(out, "--ingress tcp:4242:127.0.0.1:8080"))
	require.True(strings.Contains(out, fmt.Sprintf("--egress tcp:80:%s:4242", node1IP)))

	// Restarting the proxy...
	_, err = helper.containerExec(ctx, node1, []string{"killall", "nexd"})
	require.NoError(err)
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "proxy")
	err = helper.nexdStatus(ctx, node1)
	require.NoError(err)

	// Rules are still present
	out, err = helper.containerExec(ctx, node1, []string{"nexctl", "nexd", "proxy", "list"})
	require.NoError(err)
	require.True(strings.Contains(out, "--ingress tcp:4242:127.0.0.1:8080"))
	require.True(strings.Contains(out, fmt.Sprintf("--egress tcp:80:%s:4242", node1IP)))

	// Check connectivity through the proxy loopback
	// curl -> localhost port 80 -> nexd proxy egress rule listening on port 80 -> nexd proxy ingress rule listening on port 4242 -> python http server on port 8080
	out, err = helper.containerExec(ctx, node1, []string{"curl", "http://127.0.0.1"})
	require.NoError(err)
	require.True(strings.Contains(out, "waffles"))

	// Remove the rules
	_, err = helper.containerExec(ctx, node1, []string{"nexctl", "nexd", "proxy", "remove",
		"--ingress", "tcp:4242:127.0.0.1:8080", "--egress", fmt.Sprintf("tcp:80:%s:4242", node1IP)})
	require.NoError(err)

	// Back to no rules
	out, err = helper.containerExec(ctx, node1, []string{"nexctl", "nexd", "proxy", "list"})
	require.NoError(err)
	require.Equal(out, "")

	// Rules are not stored.
	data, err = helper.containerExec(ctx, node1, []string{"cat", "/var/lib/nexd/state.json"})
	require.NoError(err)
	s = state.State{}
	err = json.Unmarshal([]byte(data), &s)
	require.NoError(err)
	require.Equal(state.ProxyRulesConfig{
		Egress:  nil,
		Ingress: nil,
	}, s.ProxyRulesConfig)

	// Connectivity should now fail again
	_, err = helper.containerExec(ctx, node1, []string{"curl", "http://127.0.0.1"})
	require.Error(err)

	_, _ = helper.containerExec(ctx, node1, []string{"killall", "python3"})
	wg.Wait()
}

// TestProxyNexctlConnections ensures that `nexctl connections` works for `nexd proxy`
func TestProxyNexctlConnections(t *testing.T) {
	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "proxy")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy")
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	out, err := helper.containerExec(ctx, node1, []string{"nexctl", "nexd", "peers", "ping"})
	require.NoError(err)
	require.False(strings.Contains(out, "Unreachable"))
	require.True(strings.Contains(out, "Reachable"))
}

type testProxyLoadBalancerOpts struct {
	name           string
	workloadPlacer func(node1, node2 testcontainers.Container) (serverNode testcontainers.Container, clientNode testcontainers.Container)
	upstreamIP     func(node1IP, localhost string) string
	dialIP         func(node2IP, localhost string) string
	flag           string
}

// TestProxyEgress tests that nexd proxy can load balance between multiple egress rules on the same listen port
func testProxyLoadBalancer(t *testing.T, opts testProxyLoadBalancerOpts) {

	t.Parallel()
	helper := NewHelper(t)
	require := helper.require
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay")
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getTunnelIP(ctx, helper, inetV4, node1)
	require.NoError(err)

	node1IPv6, err := getTunnelIP(ctx, helper, inetV6, node1)
	require.NoError(err)

	serverNode, clientNode := opts.workloadPlacer(node1, node2)

	upstreamIP := opts.upstreamIP(node1IP, "127.0.0.1")
	upstreamIPv6 := opts.upstreamIP(node1IPv6, "::1")

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		opts.flag, fmt.Sprintf("tcp:80:%s", net.JoinHostPort(upstreamIP, "8080")),
		opts.flag, fmt.Sprintf("tcp:80:%s", net.JoinHostPort(upstreamIPv6, "8080")),
		opts.flag, fmt.Sprintf("tcp:80:%s", net.JoinHostPort(upstreamIP, "8081")),
		opts.flag, fmt.Sprintf("tcp:80:%s", net.JoinHostPort(upstreamIPv6, "8081")),
		opts.flag, fmt.Sprintf("udp:42:%s", net.JoinHostPort(upstreamIP, "4240")),
		opts.flag, fmt.Sprintf("udp:42:%s", net.JoinHostPort(upstreamIPv6, "4240")),
		opts.flag, fmt.Sprintf("udp:42:%s", net.JoinHostPort(upstreamIP, "4241")),
		opts.flag, fmt.Sprintf("udp:42:%s", net.JoinHostPort(upstreamIPv6, "4241")),
	)
	err = helper.nexdStatus(ctx, node2)
	require.NoError(err)

	node2IP, err := getTunnelIP(ctx, helper, inetV4, node2)
	require.NoError(err)
	node2IPv6, err := getTunnelIP(ctx, helper, inetV6, node2)
	require.NoError(err)

	// ping node2 from node1 to verify basic connectivity over wireguard
	// before moving on to exercising the proxy functionality.
	helper.Logf("Pinging %s from node1", node2IP)
	err = ping(ctx, node1, inetV4, node2IP)
	require.NoError(err)

	// Start the servers on serverNode
	wg := sync.WaitGroup{}
	util.GoWithWaitGroup(&wg, func() {
		_, err = helper.containerExec(ctx, serverNode, []string{"mkdir", "-p", "/tmp/apples"})
		require.NoError(err)
		_, err := helper.containerExec(ctx, serverNode, []string{"python3", "-c", "import os; open('/tmp/apples/index.html', 'w').write('apples')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, serverNode, []string{"python3", "-m", "http.server", "-b", "::", "-d", "/tmp/apples", "8080"})
	})
	util.GoWithWaitGroup(&wg, func() {
		_, err = helper.containerExec(ctx, serverNode, []string{"mkdir", "-p", "/tmp/bananas"})
		require.NoError(err)
		_, err := helper.containerExec(ctx, serverNode, []string{"python3", "-c", "import os; open('/tmp/bananas/index.html', 'w').write('bananas')"})
		require.NoError(err)
		_, _ = helper.containerExec(ctx, serverNode, []string{"python3", "-m", "http.server", "-b", "::", "-d", "/tmp/bananas", "8081"})
	})
	util.GoWithWaitGroup(&wg, func() {
		output, _ := helper.containerExec(ctx, serverNode, []string{"udpong", "4240", "carrot"})
		helper.Logf("udpong output: %s", output)
	})
	util.GoWithWaitGroup(&wg, func() {
		_, _ = helper.containerExec(ctx, serverNode, []string{"udpong", "4241", "potato"})
	})

	ctxTimeout, curlCancel := context.WithTimeout(ctx, 10*time.Second)
	defer curlCancel()

	dialIP := ""
	sendRequests := func() (bool, error) {
		bananas := 0
		apples := 0
		carrot := 0
		potato := 0
		for i := 0; i < 10; i++ {

			output, err := helper.containerExec(ctx, clientNode, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(dialIP, "80"))})
			if err != nil {
				helper.Logf("Retrying curl for up to 10 seconds: %v -- %s", err, output)
				return false, nil
			}
			if strings.Contains(output, "bananas") {
				bananas += 1
			} else if strings.Contains(output, "apples") {
				apples += 1
			}

			output, err = helper.containerExec(ctx, clientNode, []string{"udping", dialIP, "42"})
			if err != nil {
				helper.Logf("Retrying udp client for up to 10 seconds: %v -- %s", err, output)
				return false, nil
			}
			if strings.Contains(output, "carrot") {
				carrot += 1
			} else if strings.Contains(output, "potato") {
				potato += 1
			}
		}
		require.Equal(5, apples)
		require.Equal(5, bananas)
		require.Equal(5, carrot)
		require.Equal(5, potato)
		return true, nil
	}

	// Run the IPv4 workload
	dialIP = opts.dialIP(node2IP, "127.0.0.1")
	success, err := util.CheckPeriodically(ctxTimeout, time.Second, sendRequests)
	require.NoError(err)
	require.True(success)

	// Run the IPv6 workload
	dialIP = opts.dialIP(node2IPv6, "::1")
	success, err = util.CheckPeriodically(ctxTimeout, time.Second, sendRequests)
	require.NoError(err)
	require.True(success)

	_, _ = helper.containerExec(ctx, serverNode, []string{"killall", "python3"})
	_, _ = helper.containerExec(ctx, serverNode, []string{"killall", "udpong"})
	wg.Wait()
}

func TestProxyLoadBalancer(t *testing.T) {
	tests := []testProxyLoadBalancerOpts{
		{
			name: "Egress",
			flag: "--egress",
			upstreamIP: func(node1IP, localhost string) string {
				return node1IP
			},
			dialIP: func(node2IP, localhost string) string {
				return localhost
			},
			workloadPlacer: func(node1, node2 testcontainers.Container) (serverNode testcontainers.Container, clientNode testcontainers.Container) {
				return node1, node2
			},
		},
		{
			name: "Ingress",
			flag: "--ingress",
			upstreamIP: func(node1IP, localhost string) string {
				return localhost
			},
			dialIP: func(node2IP string, localhost string) string {
				return node2IP
			},
			workloadPlacer: func(node1, node2 testcontainers.Container) (serverNode testcontainers.Container, clientNode testcontainers.Container) {
				return node2, node1
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testProxyLoadBalancer(t, test)
		})
	}
}
