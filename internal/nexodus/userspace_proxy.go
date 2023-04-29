package nexodus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/natefinch/atomic"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

type ProxyType int

const (
	ProxyTypeEgress ProxyType = iota
	ProxyTypeIngress
)

type ProxyProtocol string

const (
	proxyProtocolTCP ProxyProtocol = "tcp"
	proxyProtocolUDP ProxyProtocol = "udp"
)

type UsProxy struct {
	ruleType     ProxyType
	protocol     ProxyProtocol
	listenPort   int
	destHost     string
	destPort     int
	logger       *zap.SugaredLogger
	userspaceNet *netstack.Net
	debugTraffic bool
	proxyCtx     context.Context
	proxyCancel  context.CancelFunc
	exitChan     chan bool
	stored       bool
}

const (
	udpMaxPayloadSize = 65507
	udpTimeout        = time.Minute
)

func (proxy *UsProxy) String() string {
	var str string
	if proxy.ruleType == ProxyTypeEgress {
		str = "--egress "
	} else {
		str = "--ingress "
	}
	str += fmt.Sprintf("%s:%d:%s", proxy.protocol, proxy.listenPort,
		net.JoinHostPort(proxy.destHost, fmt.Sprintf("%d", proxy.destPort)))
	return str
}

func (proxy *UsProxy) Equal(cmp *UsProxy) bool {
	return proxy.ruleType == cmp.ruleType &&
		proxy.protocol == cmp.protocol &&
		proxy.listenPort == cmp.listenPort &&
		proxy.destHost == cmp.destHost &&
		proxy.destPort == cmp.destPort
}

func proxyTypeStr(ruleType ProxyType) (string, error) {
	switch ruleType {
	case ProxyTypeEgress:
		return "egress", nil
	case ProxyTypeIngress:
		return "ingress", nil
	default:
		return "", fmt.Errorf("Invalid proxy rule type: %d", ruleType)
	}
}

func proxyProtocol(protocol string) (ProxyProtocol, error) {
	switch strings.ToLower(protocol) {
	case "tcp":
		return proxyProtocolTCP, nil
	case "udp":
		return proxyProtocolUDP, nil
	default:
		return "", fmt.Errorf("Invalid protocol (%s)", protocol)
	}
}

func parsePort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("Invalid port (%s): %w", portStr, err)
	}
	return port, nil
}

func proxyFromRule(rule string, ruleType ProxyType) (*UsProxy, error) {
	// protocol:port:destination_ip:destination_port
	parts := strings.Split(rule, ":")
	if len(parts) < 4 {
		return nil, fmt.Errorf("Invalid proxy rule format, must specify 4 colon-separated values (%s)", rule)
	}

	protocol, err := proxyProtocol(parts[0])
	if err != nil {
		return nil, err
	}

	port, err := parsePort(parts[1])
	if err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("Invalid port (%d)", port)
	}

	// Reassemble the string so that we parse IPv6 addresses correctly
	destHostPort := strings.Join(parts[2:], ":")
	destHost, destPortStr, err := net.SplitHostPort(destHostPort)
	if err != nil {
		return nil, fmt.Errorf("Invalid destination host:port (%s): %w", destHostPort, err)
	}

	destPort, err := parsePort(destPortStr)
	if err != nil {
		return nil, err
	}
	if destPort < 1 || destPort > 65535 {
		return nil, fmt.Errorf("Invalid destination port (%d)", destPort)
	}

	return &UsProxy{
		ruleType:   ruleType,
		protocol:   protocol,
		listenPort: port,
		destHost:   destHost,
		destPort:   destPort,
	}, nil
}

func proxyToRule(p *UsProxy) string {
	// protocol:port:destination_ip:destination_port
	return fmt.Sprintf("%s:%d:%s:%d", p.protocol, p.listenPort, p.destHost, p.destPort)
}

func (ax *Nexodus) UserspaceProxyAdd(ctx context.Context, wg *sync.WaitGroup, proxyRule string, ruleType ProxyType, start bool) (*UsProxy, error) {
	typeStr, err := proxyTypeStr(ruleType)
	if err != nil {
		return nil, err
	}

	ax.logger.Debugf("Adding userspace %s proxy rule: %s", typeStr, proxyRule)

	newProxy, err := proxyFromRule(proxyRule, ruleType)
	if err != nil {
		return nil, err
	}

	newProxy.debugTraffic, _ = strconv.ParseBool(os.Getenv("NEXD_PROXY_DEBUG_TRAFFIC"))
	newProxy.logger = ax.logger.With("proxy", typeStr, "proxyRule", proxyRule)

	ax.proxyLock.Lock()
	defer ax.proxyLock.Unlock()

	if ruleType == ProxyTypeEgress {
		for _, proxy := range ax.egressProxies {
			if proxy.protocol == newProxy.protocol && proxy.listenPort == newProxy.listenPort {
				return nil, fmt.Errorf("%s port %d is already in use by another egress proxy", newProxy.protocol, newProxy.listenPort)
			}
		}
	} else {
		for _, proxy := range ax.ingressProxies {
			if proxy.protocol == newProxy.protocol && proxy.listenPort == newProxy.listenPort {
				return nil, fmt.Errorf("%s port %d is already in use by another ingress proxy", newProxy.protocol, newProxy.listenPort)
			}
		}
	}

	if ruleType == ProxyTypeEgress {
		ax.egressProxies = append(ax.egressProxies, newProxy)
	} else {
		ax.ingressProxies = append(ax.ingressProxies, newProxy)
	}

	if start {
		newProxy.Start(ctx, wg, ax.userspaceNet)
	}
	return newProxy, nil
}

func (ax *Nexodus) UserspaceProxyRemove(proxyRule string, ruleType ProxyType) (*UsProxy, error) {
	typeStr, err := proxyTypeStr(ruleType)
	if err != nil {
		return nil, err
	}

	ax.logger.Debugf("Removing userspace %s proxy rule: %s", typeStr, proxyRule)

	cmpProxy, err := proxyFromRule(proxyRule, ruleType)
	if err != nil {
		return nil, err
	}

	ax.proxyLock.Lock()
	defer ax.proxyLock.Unlock()

	if ruleType == ProxyTypeEgress {
		for i, proxy := range ax.egressProxies {
			if proxy.Equal(cmpProxy) {
				proxy.Stop()
				ax.egressProxies = append(ax.egressProxies[:i], ax.egressProxies[i+1:]...)
				return proxy, nil
			}
		}
	} else {
		for i, proxy := range ax.ingressProxies {
			if proxy.Equal(cmpProxy) {
				proxy.Stop()
				ax.ingressProxies = append(ax.ingressProxies[:i], ax.ingressProxies[i+1:]...)
				return proxy, nil
			}
		}
	}

	return nil, fmt.Errorf("No matching %s proxy rule found: %s", typeStr, proxyRule)
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

	rules := ProxyRulesConfig{}
	err = json.NewDecoder(file).Decode(&rules)
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, r := range rules.Ingress {
		proxy, err := ax.UserspaceProxyAdd(ctx, nil, r, ProxyTypeIngress, false)
		if err != nil {
			return err
		}
		proxy.stored = true
	}
	for _, r := range rules.Egress {
		proxy, err := ax.UserspaceProxyAdd(ctx, nil, r, ProxyTypeEgress, false)
		if err != nil {
			return err
		}
		proxy.stored = true
	}
	return nil
}

func (ax *Nexodus) StoreProxyRules() error {
	ax.proxyLock.Lock()
	defer ax.proxyLock.Unlock()

	rules := ProxyRulesConfig{}
	for _, proxy := range ax.egressProxies {
		if proxy.stored {
			rules.Egress = append(rules.Egress, proxyToRule(proxy))
		}
	}
	for _, proxy := range ax.ingressProxies {
		if proxy.stored {
			rules.Ingress = append(rules.Ingress, proxyToRule(proxy))
		}
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
	proxy.userspaceNet = net
	proxy.proxyCtx, proxy.proxyCancel = context.WithCancel(ctx)
	proxy.exitChan = make(chan bool)
	util.GoWithWaitGroup(wg, func() {
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
		proxy.exitChan <- true
	})
}

func (proxy *UsProxy) Stop() {
	proxy.proxyCancel()
	<-proxy.exitChan
}

func (proxy *UsProxy) run(ctx context.Context, proxyWg *sync.WaitGroup) error {
	switch proxy.protocol {
	case proxyProtocolTCP:
		return proxy.runTCP(ctx, proxyWg)
	case proxyProtocolUDP:
		return proxy.runUDP(ctx, proxyWg)
	default:
		return fmt.Errorf("Unexpected proxy protocol: %v", proxy.protocol)
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
	if udpProxy.proxy.ruleType == ProxyTypeEgress {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", udpProxy.proxy.listenPort))
		if err != nil {
			return fmt.Errorf("Failed to resolve UDP address: %w", err)
		}
		udpProxy.conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return fmt.Errorf("Failed to listen on UDP port: %w", err)
		}
	} else {
		udpProxy.goConn, err = udpProxy.proxy.userspaceNet.ListenUDP(&net.UDPAddr{Port: udpProxy.proxy.listenPort})
		if err != nil {
			return fmt.Errorf("Failed to listen on UDP port: %w", err)
		}
	}
	return nil
}

func (udpProxy *udpProxy) setReadDeadline() error {
	var err error
	// Don't block for more than a second
	if udpProxy.proxy.ruleType == ProxyTypeEgress {
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
	if udpProxy.proxy.ruleType == ProxyTypeEgress {
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
	if proxy.ruleType == ProxyTypeEgress {
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
			if proxy.ruleType == ProxyTypeEgress {
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
	if proxyConn.udpProxy.proxy.ruleType == ProxyTypeEgress {
		_, err = proxyConn.goProxyConn.Write(buf[:n])
	} else {
		_, err = proxyConn.proxyConn.Write(buf[:n])
	}
	return err
}

func (proxyConn *udpProxyConn) setReadDeadline() error {
	var err error
	// Don't block for more than a second
	if proxyConn.udpProxy.proxy.ruleType == ProxyTypeEgress {
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
	if proxyConn.udpProxy.proxy.ruleType == ProxyTypeIngress {
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
	if proxyConn.udpProxy.proxy.ruleType == ProxyTypeEgress {
		_, err = proxyConn.udpProxy.conn.WriteToUDP(buffer[:n], proxyConn.clientAddr)
	} else {
		_, err = proxyConn.udpProxy.goConn.WriteTo(buffer[:n], proxyConn.clientAddr)
	}
	return err
}

func (proxy *UsProxy) createUDPProxyConn(ctx context.Context, proxyWg *sync.WaitGroup, proxyConn *udpProxyConn) error {
	var err error

	if proxy.ruleType == ProxyTypeEgress {
		newConn, err := proxy.userspaceNet.DialUDP(nil, &net.UDPAddr{Port: proxy.destPort, IP: net.ParseIP(proxy.destHost)})
		if err != nil {
			return fmt.Errorf("Error dialing UDP proxy destination: %w", err)
		}
		proxyConn.goProxyConn = newConn
	} else {
		udpDest := net.JoinHostPort(proxy.destHost, fmt.Sprintf("%d", proxy.destPort))
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
					proxy.logger.Debug("UDP proxy connection timed out after", udpTimeout)
				}
				break loop
			default:
				if err = proxyConn.setReadDeadline(); err != nil {
					proxy.logger.Warn("Error setting UDP read deadline:", err)
					break loop
				}
				var clientAddr2 net.Addr
				n, clientAddr2, err = proxyConn.readFromUDP(buf)
				if err != nil {
					var timeoutError net.Error
					if errors.As(err, &timeoutError); timeoutError.Timeout() {
						continue
					}
					proxy.logger.Warn("Error reading from UDP client:", err)
					break loop
				}
				if proxy.debugTraffic {
					proxy.logger.Debug("Read from UDP client:", clientAddr2, n, buf[:n])
				}
				err = proxyConn.writeBackToOriginator(buf, n)
				if err != nil {
					proxy.logger.Warn("Error writing back to original UDP source:", err)
					break loop
				}
				if proxy.debugTraffic {
					proxy.logger.Debug("Wrote to UDP proxy destination:", proxyConn.goProxyConn.RemoteAddr(), n, buf[:n])
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
	if proxy.ruleType == ProxyTypeEgress {
		l, err = net.Listen(fmt.Sprintf("%v", proxy.protocol), fmt.Sprintf(":%d", proxy.listenPort))
	} else {
		l, err = proxy.userspaceNet.ListenTCP(&net.TCPAddr{Port: proxy.listenPort})
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

	proxyDest := net.JoinHostPort(proxy.destHost, fmt.Sprintf("%d", proxy.destPort))
	proxy.logger.Debugf("Handling connection from %s, proxying to %s", inConn.RemoteAddr().String(), proxyDest)

	var outConn net.Conn
	var err error
	protocolStr := fmt.Sprintf("%v", proxy.protocol)
	if proxy.ruleType == ProxyTypeEgress {
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
			proxy.logger.Debugf("Error copying data from outConn to inConn: ", err)
		}
	})
	_, err = io.Copy(outConn, inConn)
	if err != nil {
		proxy.logger.Debugf("Error copying data from inConn to outConn: ", err)
	}

	return nil
}
