package nexodus

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type ProxyType int

const (
	ProxyTypeEgress ProxyType = iota
	ProxyTypeIngress
)

func (ruleType ProxyType) String() string {
	switch ruleType {
	case ProxyTypeEgress:
		return "egress"
	case ProxyTypeIngress:
		return "ingress"
	default:
		panic(fmt.Sprintf("Invalid proxy rule type: %d", ruleType))
	}
}

type ProxyProtocol string

const (
	proxyProtocolTCP ProxyProtocol = "tcp"
	proxyProtocolUDP ProxyProtocol = "udp"
)

func parseProxyProtocol(protocol string) (ProxyProtocol, error) {
	switch strings.ToLower(protocol) {
	case "tcp":
		return proxyProtocolTCP, nil
	case "udp":
		return proxyProtocolUDP, nil
	default:
		return "", fmt.Errorf("invalid protocol (%s)", protocol)
	}
}

type ProxyKey struct {
	ruleType   ProxyType
	protocol   ProxyProtocol
	listenPort int
}

func (rule ProxyKey) String() string {
	return fmt.Sprintf("%s:%s:%d", rule.ruleType, rule.protocol, rule.listenPort)
}

type ProxyRule struct {
	ProxyKey
	dest   HostPort
	stored bool
}

type HostPort struct {
	host string
	port int
}

func (hp HostPort) String() string {
	// destination_ip:destination_port
	return net.JoinHostPort(hp.host, fmt.Sprintf("%d", hp.port))
}

func parseHostPort(address string) (result HostPort, err error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return result, err
	}
	p, err := parsePort(port)
	if err != nil {
		return result, err
	}
	result.host = host
	result.port = p
	return result, nil
}

func (rule ProxyRule) String() string {
	// protocol:port:destination_ip:destination_port
	return fmt.Sprintf("%s:%d:%s", rule.protocol, rule.listenPort, rule.dest)
}

func (rule ProxyRule) AsFlag() string {
	return fmt.Sprintf("--%s %s", rule.ruleType, rule)
}

func parsePort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port (%s): %w", portStr, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port (%d): out of range 0-65535", port)
	}
	return port, nil
}

func ParseProxyRule(rule string, ruleType ProxyType) (emptyRule ProxyRule, err error) {
	// protocol:port:destination_ip:destination_port
	parts := strings.Split(rule, ":")
	if len(parts) < 4 {
		return emptyRule, fmt.Errorf("invalid proxy rule format, must specify 4 colon-separated values (%s)", rule)
	}

	protocol, err := parseProxyProtocol(parts[0])
	if err != nil {
		return emptyRule, err
	}

	port, err := parsePort(parts[1])
	if err != nil {
		return emptyRule, err
	}

	// Reassemble the string so that we parse IPv6 addresses correctly
	destHostPort := strings.Join(parts[2:], ":")
	destHost, destPortStr, err := net.SplitHostPort(destHostPort)
	if err != nil {
		return emptyRule, fmt.Errorf("invalid destination host:port (%s): %w", destHostPort, err)
	}

	if destHost == "" {
		return emptyRule, fmt.Errorf("invalid destination host:port (%s): host cannot be empty", destHostPort)
	}

	destPort, err := parsePort(destPortStr)
	if err != nil {
		return emptyRule, err
	}

	return ProxyRule{
		ProxyKey: ProxyKey{
			ruleType:   ruleType,
			protocol:   protocol,
			listenPort: port,
		},
		dest: HostPort{
			host: destHost,
			port: destPort,
		},
	}, nil
}
