package nexodus

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/tun/netstack"
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
		// TODO
		return proxyProtocolUDP, fmt.Errorf("UDP proxy support not yet implemented")
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

	proxy := &UsProxy{
		ruleType:   ruleType,
		protocol:   protocol,
		listenPort: port,
		destHost:   destHost,
		destPort:   destPort,
		logger:     ax.logger.With("proxy", typeStr, "proxyRule", proxyRule),
	}

	if ruleType == ProxyTypeEgress {
		ax.egressProxies = append(ax.egressProxies, proxy)
	} else {
		ax.ingresProxies = append(ax.ingresProxies, proxy)
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
