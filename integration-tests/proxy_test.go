//go:build integration

package integration_tests

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay", "--enable-discovery")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy", "--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

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
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay", "--enable-discovery")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")),
		"--egress", fmt.Sprintf("tcp:81:%s", net.JoinHostPort(node1IP, "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

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
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		output2, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost:81"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output2)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		require.True(strings.Contains(output2, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay", "--enable-discovery")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy", "--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay", "--enable-discovery")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")),
		"--ingress", fmt.Sprintf("tcp:8081:%s", net.JoinHostPort("127.0.0.1", "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

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
		output2, err := helper.containerExec(ctx, node1, []string{"curl", "-s", fmt.Sprintf("http://%s", net.JoinHostPort(node2IP, "8081"))})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output2)
			return false, nil
		}
		require.True(strings.Contains(output, "bananas"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
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
	helper.runNexd(ctx, node1, "--username", username, "--password", password, "relay", "--enable-discovery")

	// validate nexd has started on the discovery node
	err := helper.nexdStatus(ctx, node1)
	require.NoError(err)

	node1IP, err := getContainerIfaceIP(ctx, inetV4, "wg0", node1)
	require.NoError(err)

	helper.runNexd(ctx, node2, "--username", username, "--password", password, "proxy",
		"--ingress", fmt.Sprintf("tcp:8080:%s", net.JoinHostPort("127.0.0.1", "8080")),
		"--egress", fmt.Sprintf("tcp:80:%s", net.JoinHostPort(node1IP, "8080")))

	// TODO - This makes an assumption about ipam behavior that could change. We can't read the IP address
	// from "wg0" for the proxy case as there's no wg0 interface. We need a new nexctl command to read the
	// IP address from the running nexd.
	node2IP := "100.100.0.2"

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
		output, err := helper.containerExec(ctx, node2, []string{"curl", "-s", "http://localhost"})
		if err != nil {
			helper.Logf("Retrying curl for up to 10 seconds while waiting for peering to finish: %v -- %s", err, output)
			return false, nil
		}
		require.True(strings.Contains(output, "pancakes"))
		return true, nil
	})
	require.NoError(err)
	require.True(success)
	wg.Wait()
}
