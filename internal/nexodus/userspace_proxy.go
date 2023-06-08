package nexodus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/gopkg/util/logger"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/natefinch/atomic"

	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

type UsProxy struct {
	key               ProxyKey
	logger            *zap.SugaredLogger
	debugTraffic      bool
	mu                sync.RWMutex
	rules             []ProxyRule
	connectionCounter uint64
	userspaceNet      *netstack.Net
	proxyCtx          context.Context
	proxyCancel       context.CancelFunc
	wg                sync.WaitGroup
}

const (
	udpMaxPayloadSize = 65507
	udpTimeout        = time.Minute
)

var ProxyExistsError = errors.New("port already in use by another proxy rule")

func (ax *Nexodus) UserspaceProxyAdd(newRule ProxyRule) (*UsProxy, error) {

	ax.logger.Debugf("Adding userspace %s proxy rule: %s", newRule.ruleType, newRule)

	ax.proxyLock.Lock()
	defer ax.proxyLock.Unlock()

	proxy, found := ax.proxies[newRule.ProxyKey]
	if !found {
		proxy = &UsProxy{
			key:    newRule.ProxyKey,
			logger: ax.logger.With("proxy", newRule.ruleType, "key", newRule.ProxyKey),
		}
		proxy.debugTraffic, _ = strconv.ParseBool(os.Getenv("NEXD_PROXY_DEBUG_TRAFFIC"))
		ax.proxies[newRule.ProxyKey] = proxy
	}

	for _, rule := range proxy.rules {
		if rule == newRule {
			return proxy, ProxyExistsError
		}
	}

	proxy.mu.Lock()
	defer proxy.mu.Unlock()

	proxy.rules = append(proxy.rules, newRule)
	return proxy, nil
}

func (ax *Nexodus) UserspaceProxyRemove(cmpProxy ProxyRule) (*UsProxy, error) {

	ax.logger.Debugf("Removing userspace %s proxy rule: %s", cmpProxy.ruleType, cmpProxy)

	ax.proxyLock.Lock()
	defer ax.proxyLock.Unlock()

	proxy, found := ax.proxies[cmpProxy.ProxyKey]
	if !found {
		return nil, fmt.Errorf("no matching %s proxy rule found: %s", cmpProxy.ruleType, cmpProxy)
	}

	proxy.mu.Lock()
	defer proxy.mu.Unlock()

	for i, rule := range proxy.rules {
		if rule == cmpProxy {
			proxy.rules = append(proxy.rules[:i], proxy.rules[i+1:]...)
			if len(proxy.rules) == 0 {
				proxy.Stop()
				delete(ax.proxies, cmpProxy.ProxyKey)
			}
			return proxy, nil
		}
	}
	return nil, fmt.Errorf("no matching %s proxy rule found: %s", cmpProxy.ruleType, cmpProxy)
}

type ProxyRulesConfig struct {
	Egress  []string `json:"egress"`
	Ingress []string `json:"ingress"`
}

func (ax *Nexodus) LoadProxyRules() error {

	fileName := filepath.Join(ax.stateDir, "proxy-rules.json")
	// don't load if file does not exist...
	if _, err := os.Stat(fileName); err != nil {
		return nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	rules := ProxyRulesConfig{}
	err = json.NewDecoder(file).Decode(&rules)
	if err != nil {
		return err
	}

	parseAndAdd := func(rules []string, proxyType ProxyType) error {
		for _, r := range rules {
			rule, err := ParseProxyRule(r, proxyType)
			if err != nil {
				logger.Fatal(fmt.Sprintf("Failed to parse %s proxy rule (%s): %v", proxyType, r, err))
			}
			rule.stored = true

			_, err = ax.UserspaceProxyAdd(rule)
			if err != nil {
				if !errors.Is(err, ProxyExistsError) {
					return err
				}
			}
		}
		return nil
	}
	err = parseAndAdd(rules.Ingress, ProxyTypeIngress)
	if err != nil {
		return nil
	}
	err = parseAndAdd(rules.Egress, ProxyTypeEgress)
	if err != nil {
		return nil
	}
	return nil
}

func (ax *Nexodus) StoreProxyRules() error {
	ax.proxyLock.Lock()
	defer ax.proxyLock.Unlock()

	rules := ProxyRulesConfig{}
	for _, proxy := range ax.proxies {
		proxy.mu.Lock()
		for _, rule := range proxy.rules {
			if rule.stored {
				if rule.ruleType == ProxyTypeEgress {
					rules.Egress = append(rules.Egress, rule.String())
				} else {
					rules.Ingress = append(rules.Ingress, rule.String())
				}
			}
		}
		proxy.mu.Unlock()
	}

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	err := enc.Encode(rules)
	if err != nil {
		return err
	}
	err = atomic.WriteFile(filepath.Join(ax.stateDir, "proxy-rules.json"), buf)
	if err != nil {
		return err
	}
	return nil
}

func (proxy *UsProxy) Start(ctx context.Context, wg *sync.WaitGroup, net *netstack.Net) {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.proxyCtx != nil {
		return
	}
	proxy.proxyCtx, proxy.proxyCancel = context.WithCancel(ctx)
	proxy.userspaceNet = net
	proxy.wg.Add(1)
	util.GoWithWaitGroup(wg, func() {
		defer proxy.wg.Done()
		for {
			// Use a different waitgroup here, because we want to make sure
			// all of the subroutines have exited before we attempt to restart
			// the proxy listener.
			proxyWg := &sync.WaitGroup{}
			err := proxy.run(proxy.proxyCtx, proxyWg)
			proxyWg.Wait()
			if err == nil {
				// No error means it shut down cleanly because it got a message to stop
				break
			}
			proxy.logger.Debug("Proxy error, restarting: ", err)
			time.Sleep(time.Second)
		}
	})

}

func (proxy *UsProxy) Stop() {
	proxy.proxyCancel()
	proxy.wg.Wait()
}

func (proxy *UsProxy) run(ctx context.Context, proxyWg *sync.WaitGroup) error {
	switch proxy.key.protocol {
	case proxyProtocolTCP:
		return proxy.runTCP(ctx, proxyWg)
	case proxyProtocolUDP:
		return proxy.runUDP(ctx, proxyWg)
	default:
		return fmt.Errorf("unexpected proxy protocol: %v", proxy.key.protocol)
	}
}

// An instance of a UDP proxy.
// ingress or egress is determined by looking at the parent UsProxy
type udpProxy struct {
	// Parent UsProxy
	proxy *UsProxy
	// Listener egress proxy
	conn *net.UDPConn
	// Listener for ingress proxy
	goConn *gonet.UDPConn
}

// State tracking for a flow initiated to the UDP proxy,
// including the connection to the destination.
type udpProxyConn struct {
	// Parent udpProxy
	udpProxy *udpProxy
	// The client that originated a stream to the UDP proxy
	clientAddr *net.UDPAddr
	// The connection with the destination for an egress proxy
	goProxyConn *gonet.UDPConn
	// The connection with the destination for an ingress proxy
	proxyConn *net.UDPConn
	// Notify when the connection is to be closed
	closeChan chan string
	// track the last time inbound traffic was received
	lastActivity time.Time
}

func (udpProxy *udpProxy) setupListener() error {
	var err error
	if udpProxy.proxy.key.ruleType == ProxyTypeEgress {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", udpProxy.proxy.key.listenPort))
		if err != nil {
			return fmt.Errorf("Failed to resolve UDP address: %w", err)
		}
		udpProxy.conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return fmt.Errorf("Failed to listen on UDP port: %w", err)
		}
	} else {
		udpProxy.goConn, err = udpProxy.proxy.userspaceNet.ListenUDP(&net.UDPAddr{Port: udpProxy.proxy.key.listenPort})
		if err != nil {
			return fmt.Errorf("Failed to listen on UDP port: %w", err)
		}
	}
	return nil
}

func (udpProxy *udpProxy) setReadDeadline() error {
	var err error
	// Don't block for more than a second
	if udpProxy.proxy.key.ruleType == ProxyTypeEgress {
		err = udpProxy.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	} else {
		err = udpProxy.goConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	}
	return err
}

func (udpProxy *udpProxy) readFromUDP(buffer []byte) (int, *net.UDPAddr, error) {
	var n int
	var clientAddr *net.UDPAddr
	var err error
	if udpProxy.proxy.key.ruleType == ProxyTypeEgress {
		n, clientAddr, err = udpProxy.conn.ReadFromUDP(buffer)
	} else {
		var netAddr net.Addr
		n, netAddr, err = udpProxy.goConn.ReadFrom(buffer)
		if err == nil {
			clientAddr, err = net.ResolveUDPAddr("udp", netAddr.String())
		}
	}
	return n, clientAddr, err
}

func (proxy *UsProxy) runUDP(ctx context.Context, proxyWg *sync.WaitGroup) error {
	var err error
	udpProxy := &udpProxy{proxy: proxy}

	if err = udpProxy.setupListener(); err != nil {
		return err
	}
	if proxy.key.ruleType == ProxyTypeEgress {
		defer udpProxy.conn.Close()
	} else {
		defer udpProxy.goConn.Close()
	}

	buffer := make([]byte, udpMaxPayloadSize)
	proxyConns := make(map[string]*udpProxyConn)
	var clientAddr *net.UDPAddr
	var n int
	// a channel for being notified when a proxy connection is to be closed
	closeChan := make(chan string)
	defer close(closeChan)
	for {
		select {
		case <-ctx.Done():
			return nil
		case clientAddrStr := <-closeChan:
			// This connection has timed out, so drop our reference to it and close the connection
			proxyConn := proxyConns[clientAddrStr]
			if proxyConn == nil {
				proxy.logger.Warn("No proxy connection found for client address:", clientAddrStr)
				continue
			}
			if proxy.debugTraffic {
				proxy.logger.Debug("Closing proxy connection for client:", clientAddrStr)
			}
			if proxy.key.ruleType == ProxyTypeEgress {
				proxyConn.goProxyConn.Close()
			} else {
				proxyConn.proxyConn.Close()
			}
			delete(proxyConns, clientAddrStr)
		default:
			// read a packet from the originator sent to the proxy
			if err = udpProxy.setReadDeadline(); err != nil {
				proxy.logger.Warn("Error setting UDP read deadline:", err)
				continue
			}
			n, clientAddr, err = udpProxy.readFromUDP(buffer)
			if err != nil {
				var timeoutError net.Error
				if errors.As(err, &timeoutError); timeoutError.Timeout() {
					continue
				}
				proxy.logger.Warn("Error reading from UDP client:", err)
				continue
			}
			if proxy.debugTraffic {
				proxy.logger.Debug("Read from UDP client:", clientAddr, n, buffer[:n])
			}

			// find the appropriate destination connection for this packet.
			// It may be a new connection, or an existing one.
			var proxyConn *udpProxyConn
			if proxyConn = proxyConns[clientAddr.String()]; proxyConn == nil {
				// new connection, start a goroutine to handle packets in the reverse direction
				proxyConn = &udpProxyConn{udpProxy: udpProxy, clientAddr: clientAddr, closeChan: closeChan}
				err = proxy.createUDPProxyConn(ctx, proxyWg, proxyConn)
				if err != nil {
					proxy.logger.Warn("Error creating UDP proxy connection:", err)
					continue
				}
				proxyConns[clientAddr.String()] = proxyConn
			}

			// forward the original packet to the destination
			err = proxyConn.writeToDestination(buffer, n)
			if err != nil {
				proxy.logger.Warn("Error writing to UDP proxy destination:", err)
			} else if proxy.debugTraffic {
				proxy.logger.Info("Wrote to UDP proxy destination:", proxyConn.goProxyConn.RemoteAddr(), n, buffer[:n])
			}

			// keep track of the last time we saw a packet in this direction.
			// This is used to help determine when to time out the connection
			// and remove it from our cache (proxyConns).
			proxyConn.lastActivity = time.Now()
		}
	}
}

func (proxyConn *udpProxyConn) writeToDestination(buf []byte, n int) error {
	var err error
	if proxyConn.udpProxy.proxy.key.ruleType == ProxyTypeEgress {
		_, err = proxyConn.goProxyConn.Write(buf[:n])
	} else {
		_, err = proxyConn.proxyConn.Write(buf[:n])
	}
	return err
}

func (proxyConn *udpProxyConn) setReadDeadline() error {
	var err error
	// Don't block for more than a second
	if proxyConn.udpProxy.proxy.key.ruleType == ProxyTypeEgress {
		err = proxyConn.goProxyConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	} else {
		err = proxyConn.proxyConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	}
	return err
}

func (proxyConn *udpProxyConn) readFromUDP(buffer []byte) (int, *net.UDPAddr, error) {
	var n int
	var clientAddr *net.UDPAddr
	var err error
	if proxyConn.udpProxy.proxy.key.ruleType == ProxyTypeIngress {
		n, clientAddr, err = proxyConn.proxyConn.ReadFromUDP(buffer)
	} else {
		var netAddr net.Addr
		n, netAddr, err = proxyConn.goProxyConn.ReadFrom(buffer)
		if err == nil {
			clientAddr, err = net.ResolveUDPAddr("udp", netAddr.String())
		}
	}
	return n, clientAddr, err
}

func (proxyConn *udpProxyConn) writeBackToOriginator(buffer []byte, n int) error {
	var err error
	if proxyConn.udpProxy.proxy.key.ruleType == ProxyTypeEgress {
		_, err = proxyConn.udpProxy.conn.WriteToUDP(buffer[:n], proxyConn.clientAddr)
	} else {
		_, err = proxyConn.udpProxy.goConn.WriteTo(buffer[:n], proxyConn.clientAddr)
	}
	return err
}

func (proxy *UsProxy) NextDest() HostPort {
	proxy.mu.RLock()
	defer proxy.mu.RUnlock()

	proxy.connectionCounter += 1

	index := proxy.connectionCounter % uint64(len(proxy.rules))
	return proxy.rules[index].dest
}

func (proxy *UsProxy) createUDPProxyConn(ctx context.Context, proxyWg *sync.WaitGroup, proxyConn *udpProxyConn) error {
	var err error
	dest := proxy.NextDest()
	logger := proxy.logger.With("dest", dest)

	if proxy.key.ruleType == ProxyTypeEgress {
		newConn, err := proxy.userspaceNet.DialUDP(nil, &net.UDPAddr{Port: dest.port, IP: net.ParseIP(dest.host)})
		if err != nil {
			return fmt.Errorf("Error dialing UDP proxy destination: %w", err)
		}
		proxyConn.goProxyConn = newConn
	} else {
		udpDest := net.JoinHostPort(dest.host, fmt.Sprintf("%d", dest.port))
		addr, err := net.ResolveUDPAddr("udp", udpDest)
		if err != nil {
			return fmt.Errorf("Failed to resolve UDP address: %w", err)
		}
		proxyConn.proxyConn, err = net.DialUDP("udp", nil, addr)
		if err != nil {
			return fmt.Errorf("Failed to Dial UDP destination %s: %w", udpDest, err)
		}
	}

	// Start a goroutine to handle proxying data from the destination back to the client.
	util.GoWithWaitGroup(proxyWg, func() {
		buf := make([]byte, udpMaxPayloadSize)
		var n int
		// Handle proxying data from the destination back to the client.
		// There is a different UDPConn per stream here because we use
		// a different UDP source port for each stream.
		timer := time.NewTimer(udpTimeout)
	loop:
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				// The connection timed out based only on return traffic activity.
				// Check to see if any traffic from the originating side has happened
				// within the timeout period.
				if time.Since(proxyConn.lastActivity) < udpTimeout {
					timer.Reset(udpTimeout - time.Since(proxyConn.lastActivity))
					continue
				}
				if proxy.debugTraffic {
					logger.Debug("UDP proxy connection timed out after", udpTimeout)
				}
				break loop
			default:
				if err = proxyConn.setReadDeadline(); err != nil {
					logger.Warn("Error setting UDP read deadline:", err)
					break loop
				}
				var clientAddr2 net.Addr
				n, clientAddr2, err = proxyConn.readFromUDP(buf)
				if err != nil {
					var timeoutError net.Error
					if errors.As(err, &timeoutError); timeoutError.Timeout() {
						continue
					}
					logger.Warn("Error reading from UDP client:", err)
					break loop
				}
				if proxy.debugTraffic {
					logger.Debug("Read from UDP client:", clientAddr2, n, buf[:n])
				}
				err = proxyConn.writeBackToOriginator(buf, n)
				if err != nil {
					logger.Warn("Error writing back to original UDP source:", err)
					break loop
				}
				if proxy.debugTraffic {
					logger.Debug("Wrote to UDP proxy destination:", proxyConn.goProxyConn.RemoteAddr(), n, buf[:n])
				}
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(udpTimeout)
			}
		}
		proxyConn.closeChan <- proxyConn.clientAddr.String()
	})

	return nil
}

func (proxy *UsProxy) runTCP(ctx context.Context, proxyWg *sync.WaitGroup) error {
	var l net.Listener
	var err error
	if proxy.key.ruleType == ProxyTypeEgress {
		l, err = net.Listen(fmt.Sprintf("%v", proxy.key.protocol), fmt.Sprintf(":%d", proxy.key.listenPort))
	} else {
		l, err = proxy.userspaceNet.ListenTCP(&net.TCPAddr{Port: proxy.key.listenPort})
	}
	if err != nil {
		proxy.logger.Error("Error creating listener: ", err)
		return err
	}
	defer l.Close()

	// This routine will exit when the listener is closed intentionally,
	// or some error occurs.
	errChan := make(chan error)
	util.GoWithWaitGroup(proxyWg, func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				// Don't return an error if the context was canceled
				if ctx.Err() == nil {
					errChan <- err
				}
				break
			}
			util.GoWithWaitGroup(proxyWg, func() {
				err = proxy.handleTCPConnection(ctx, proxyWg, conn)
				proxy.logger.Debugf("Connection from %s closed: %v", conn.RemoteAddr().String(), err)
			})
		}
	})

	// Handle new connections until we get notified to stop the CtlServer,
	// or Accept() fails for some reason.
	stopNow := false
	for {
		select {
		case err = <-errChan:
			// Accept() failed, collect the error and stop the CtlServer
			stopNow = true
			proxy.logger.Error("Error on Accept(): ", err)
			break
		case <-ctx.Done():
			proxy.logger.Info("Stopping proxy due to context cancel")
			stopNow = true
			err = nil
		}
		if stopNow {
			break
		}
	}

	return err
}

func (proxy *UsProxy) handleTCPConnection(ctx context.Context, proxyWg *sync.WaitGroup, inConn net.Conn) error {
	defer inConn.Close()

	dest := proxy.NextDest()
	logger := proxy.logger.With("dest", dest)

	proxyDest := net.JoinHostPort(dest.host, fmt.Sprintf("%d", dest.port))
	logger.Debugf("Handling connection from %s, proxying to %s", inConn.RemoteAddr().String(), proxyDest)

	var outConn net.Conn
	var err error
	protocolStr := fmt.Sprintf("%v", proxy.key.protocol)
	if proxy.key.ruleType == ProxyTypeEgress {
		outConn, err = proxy.userspaceNet.DialContext(ctx, protocolStr, proxyDest)
	} else {
		outConn, err = net.Dial(protocolStr, proxyDest)
	}
	if err != nil {
		return err
	}
	defer outConn.Close()

	util.GoWithWaitGroup(proxyWg, func() {
		_, err := io.Copy(inConn, outConn)
		if err != nil {
			logger.Debugf("Error copying data from outConn to inConn: ", err)
		}
	})
	_, err = io.Copy(outConn, inConn)
	if err != nil {
		logger.Debugf("Error copying data from inConn to outConn: ", err)
	}

	return nil
}
