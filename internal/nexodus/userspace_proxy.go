package nexodus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
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
}

const (
	udpMaxPayloadSize = 65507
	udpTimeout        = time.Minute
)

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

func (ax *Nexodus) UserspaceProxyAdd(ctx context.Context, wg *sync.WaitGroup, proxyRule string, ruleType ProxyType) error {
	typeStr, err := proxyTypeStr(ruleType)
	if err != nil {
		return err
	}

	// protocol:port:destination_ip:destination_port
	ax.logger.Debugf("Adding userspace %s proxy rule: %s", typeStr, proxyRule)
	parts := strings.Split(proxyRule, ":")
	if len(parts) < 4 {
		return fmt.Errorf("Invalid proxy rule format, must specify 4 colon-separated values (%s)", proxyRule)
	}

	protocol, err := proxyProtocol(parts[0])
	if err != nil {
		return err
	}

	port, err := parsePort(parts[1])
	if err != nil {
		return err
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("Invalid port (%d)", port)
	}

	if ruleType == ProxyTypeEgress {
		for _, proxy := range ax.egressProxies {
			if proxy.protocol == protocol && proxy.listenPort == port {
				return fmt.Errorf("%s port %d is already in use by another egress proxy", protocol, port)
			}
		}
	} else {
		for _, proxy := range ax.ingressProxies {
			if proxy.protocol == protocol && proxy.listenPort == port {
				return fmt.Errorf("%s port %d is already in use by another ingress proxy", protocol, port)
			}
		}
	}

	// Reassemble the string so that we parse IPv6 addresses correctly
	destHostPort := strings.Join(parts[2:], ":")
	destHost, destPortStr, err := net.SplitHostPort(destHostPort)
	if err != nil {
		return fmt.Errorf("Failed to parse destination host and port: %s", destHostPort)
	}
	destPort, err := parsePort(destPortStr)
	if err != nil {
		return err
	}
	if destPort < 1 || destPort > 65535 {
		return fmt.Errorf("Invalid port (%d)", destPort)
	}

	debugTraffic, _ := strconv.ParseBool(os.Getenv("NEXD_PROXY_DEBUG_TRAFFIC"))
	proxy := &UsProxy{
		ruleType:     ruleType,
		protocol:     protocol,
		listenPort:   port,
		destHost:     destHost,
		destPort:     destPort,
		logger:       ax.logger.With("proxy", typeStr, "proxyRule", proxyRule),
		debugTraffic: debugTraffic,
	}

	if ruleType == ProxyTypeEgress {
		ax.egressProxies = append(ax.egressProxies, proxy)
	} else {
		ax.ingressProxies = append(ax.ingressProxies, proxy)
	}

	return nil
}

func (proxy *UsProxy) Start(ctx context.Context, wg *sync.WaitGroup, net *netstack.Net) {
	proxy.userspaceNet = net
	util.GoWithWaitGroup(wg, func() {
		for {
			// Use a different waitgroup here, because we want to make sure
			// all of the subroutines have exited before we attempt to restart
			// the proxy listener.
			proxyWg := &sync.WaitGroup{}
			err := proxy.run(ctx, proxyWg)
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

func (proxy *UsProxy) runUDP(ctx context.Context, proxyWg *sync.WaitGroup) error {
	switch proxy.ruleType {
	case ProxyTypeEgress:
		return proxy.runUDPEgress(ctx, proxyWg)
	case ProxyTypeIngress:
		return proxy.runUDPIngress(ctx, proxyWg)
	default:
		return fmt.Errorf("Unexpected proxy rule type: %v", proxy.ruleType)
	}
}

type udpProxyConn struct {
	// The client that originated a stream to the UDP proxy
	clientAddr *net.UDPAddr
	// The connection with the originator
	conn *net.UDPConn
	// The connection with the destination
	proxyConn *gonet.UDPConn
	// Notify when the connection is to be closed
	closeChan chan string
	// track the last time inbound traffic was received
	lastActivity time.Time
}

func (proxy *UsProxy) runUDPEgress(ctx context.Context, proxyWg *sync.WaitGroup) error {
	var conn *net.UDPConn
	var err error

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", proxy.listenPort))
	if err != nil {
		return fmt.Errorf("Failed to resolve UDP address: %w", err)
	}
	conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen on UDP port: %w", err)
	}
	defer conn.Close()

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
			proxyConn.proxyConn.Close()
			delete(proxyConns, clientAddrStr)
		default:
			// Read from the UDP socket, but don't block for more than a second
			if err = conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				proxy.logger.Warn("Error setting UDP read deadline:", err)
				continue
			}
			n, clientAddr, err = conn.ReadFromUDP(buffer)
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

			var proxyConn *udpProxyConn
			if proxyConn = proxyConns[clientAddr.String()]; proxyConn == nil {
				proxyConn = &udpProxyConn{conn: conn, clientAddr: clientAddr, closeChan: closeChan}
				err = proxy.createUDPProxyConn(ctx, proxyWg, proxyConn)
				if err != nil {
					proxy.logger.Warn("Error creating UDP proxy connection:", err)
					continue
				}
				proxyConns[clientAddr.String()] = proxyConn
			}
			_, err = proxyConn.proxyConn.Write(buffer[:n])
			if err != nil {
				proxy.logger.Warn("Error writing to UDP proxy destination:", err)
			} else if proxy.debugTraffic {
				proxy.logger.Info("Wrote to UDP proxy destination:", proxyConn.proxyConn.RemoteAddr(), n, buffer[:n])
			}
			proxyConn.lastActivity = time.Now()
		}
	}
}

func (proxy *UsProxy) createUDPProxyConn(ctx context.Context, proxyWg *sync.WaitGroup, proxyConn *udpProxyConn) error {
	newConn, err := proxy.userspaceNet.DialUDP(nil, &net.UDPAddr{Port: proxy.destPort, IP: net.ParseIP(proxy.destHost)})
	if err != nil {
		return fmt.Errorf("Error dialing UDP proxy destination: %w", err)
	}
	proxyConn.proxyConn = newConn

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
				if err = proxyConn.proxyConn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
					proxy.logger.Warn("Error setting UDP read deadline:", err)
					break loop
				}
				var clientAddr2 net.Addr
				n, clientAddr2, err = proxyConn.proxyConn.ReadFrom(buf)
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
				_, err = proxyConn.conn.WriteToUDP(buf[:n], proxyConn.clientAddr)
				if err != nil {
					proxy.logger.Warn("Error writing back to original UDP source:", err)
					break loop
				}
				if proxy.debugTraffic {
					proxy.logger.Debug("Wrote to UDP proxy destination:", proxyConn.proxyConn.RemoteAddr(), n, buf[:n])
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

func (proxy *UsProxy) runUDPIngress(ctx context.Context, proxyWg *sync.WaitGroup) error {
	// goConn, err = proxy.userspaceNet.ListenUDP(&net.UDPAddr{Port: proxy.listenPort})
	// if err != nil {
	// 	return fmt.Errorf("Failed to listen on UDP port: %w", err)
	// }
	// defer goConn.Close()

	// TODO
	return fmt.Errorf("Ingress UDP proxy not implemented yet")
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
				err = proxy.handleConnection(ctx, proxyWg, conn)
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

func (proxy *UsProxy) handleConnection(ctx context.Context, proxyWg *sync.WaitGroup, inConn net.Conn) error {
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
