package nexodus

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// Nftables keywords
	sgTableName  = "nexodus"
	tableFamily  = "inet"
	ingressChain = "nexodus-inbound"
	egressChain  = "nexodus-outbound"
	destPort     = "dport"
	destAddr     = "daddr"
	srcAddr      = "saddr"
	actionAccept = "accept"
	actionDrop   = "drop"
	counter      = "counter"
	// Protocols
	protoIPv4   = "ipv4"
	protoIPv6   = "ipv6"
	protoICMPv4 = "icmpv4"
	protoICMP   = "icmp"
	protoICMPv6 = "icmpv6"
	protoTCP    = "tcp"
	protoUDP    = "udp"
	// Network router keywords
	rtrTableName     = "nexodus-net-router"
	chainPrerouting  = "prerouting"
	chainPostrouting = "postrouting"
	chainForward     = "forward"
	chainTypeNAT     = "nat"
	chainTypeFilter  = "filter"
	priorityDstNAT   = "dstnat"
	prioritySrcNAT   = "srcnat"
	priorityFilter   = "filter"
)

var (
	ruleInterface string
)

// processSecurityGroupRules processes a security group for a Linux node
func (nx *Nexodus) processSecurityGroupRules() error {

	// Delete the table if the security group is empty and attempt to drop a table if one exists
	if nx.securityGroup == nil {
		// Drop the existing table and return nil if a group was not found to drop
		_ = nx.nfTableDrop(sgTableName)
		return nil
	}

	ruleInterface = fmt.Sprintf("iifname %s", wgIface)

	inboundRules := nx.securityGroup.InboundRules
	outboundRules := nx.securityGroup.OutboundRules

	// Enable rule debugging to print rules via debug logging as they are processed
	if nx.logger.Level().Enabled(zapcore.DebugLevel) {
		err := debugSecurityGroupRules(nx.logger, inboundRules, outboundRules)
		if err != nil {
			nx.logger.Debug(err)
		}
	}

	// Drop the existing table
	if err := nx.nfTableDrop(sgTableName); err != nil {
		return fmt.Errorf("nftables setup error, failed to flush nftables: %w", err)
	}

	// Create the nftables table
	if err := nx.nfCreateTable(sgTableName); err != nil {
		return fmt.Errorf("nftables setup error, failed to create nftables inet table: %w", err)
	}

	// Create the ingress nftables chains
	if err := nx.nfCreateChain(ingressChain); err != nil {
		return fmt.Errorf("nftables setup error, failed to create nftables chain %s: %w", ingressChain, err)
	}

	// Create the egress nftables chains
	if err := nx.nfCreateChain(egressChain); err != nil {
		return fmt.Errorf("nftables setup error, failed to create nftables chain %s: %w", egressChain, err)
	}

	// Process the inbound rules
	for _, rule := range inboundRules {
		if len(rule.IpRanges) == 0 { // If the ip range is empty, add one
			rule.IpRanges = append(rule.IpRanges, "")
		}
		if containsIPv4Range(rule.IpRanges) {
			// if the rule is a L3 addresses in v4 family, with or without L4 port(s)
			if err := nx.nfPermitProtoPortAddrV4(ingressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process inbound v4 rule: %w", err)
			}
		} else if containsIPv6Range(rule.IpRanges) {
			// if the rule is a L3 addresses in v6 family, with or without L4 port(s)
			if err := nx.nfPermitProtoPortAddrV6(ingressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process inbound v6 rule: %w", err)
			}
		} else if rule.FromPort != 0 && rule.ToPort != 0 {
			// if the rule is L4 port(s) range with no l3 addresses
			if err := nx.nfPermitProtoPort(ingressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process inbound destination port rule: %w", err)
			}
		} else {
			// if the rule is only protocol to permit (no L4 ports or L3 addresses)
			if err := nx.nfPermitProtoAny(ingressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process inbound destination port rule: %w", err)
			}
		}
	}

	// Process the outbound rules
	for _, rule := range outboundRules {
		if len(rule.IpRanges) == 0 { // If the ip range is empty, add one
			rule.IpRanges = append(rule.IpRanges, "")
		}
		if containsIPv4Range(rule.IpRanges) {
			// if the rule is a L3 addresses in v4 family, with or without L4 port(s)
			if err := nx.nfPermitProtoPortAddrV4(egressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process outbound v4 rule: %w", err)
			}
		} else if containsIPv6Range(rule.IpRanges) {
			// if the rule is a L3 addresses in v6 family, with or without L4 port(s)
			if err := nx.nfPermitProtoPortAddrV6(egressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process outbound v6 rule: %w", err)
			}
		} else if rule.FromPort != 0 && rule.ToPort != 0 {
			// if the rule is L4 port(s) range with no l3 addresses
			if err := nx.nfPermitProtoPort(egressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process inbound destination port rule: %w", err)
			}
		} else {
			// if the rule is only protocol to permit (no L4 ports or L3 addresses)
			if err := nx.nfPermitProtoAny(egressChain, rule); err != nil {
				return fmt.Errorf("nftables setup error, failed to process inbound destination port rule: %w", err)
			}
		}
	}

	// the ct module provides access to the connection tracking subsystem, which tracks the state of network
	// connections. The state keyword is used to match traffic based on its connection state, in this case as
	// established. The established state refers to traffic that is part of an existing connection that has
	// already been established, and where both endpoints have exchanged packets.
	nft := []string{"insert", "rule", tableFamily, sgTableName, ingressChain, "ct", "state", "established,related", ruleInterface, "counter", "accept"}
	if _, err := runNftCmd(nx.logger, nft); err != nil {
		return err
	}

	// append a default drop that appears implicit to the user only if there are any rules in the egress chain
	if nx.securityGroup.InboundRules != nil && len(nx.securityGroup.InboundRules) != 0 {
		if err := nx.nfIngressRuleDrop(); err != nil {
			return fmt.Errorf("nftables setup error, failed to add ingress drop rule: %w", err)
		}
	}

	// append a drop that appears implicit to the user only if there are any user defined rules in the egress chain
	if nx.securityGroup.OutboundRules != nil && len(nx.securityGroup.OutboundRules) != 0 {
		if err := nx.nfEgressRuleDrop(); err != nil {
			return fmt.Errorf("nftables setup error, failed to add egress drop rule: %w", err)
		}
	}

	return nil
}

// nfPermitProtoPortAddrV4 creates a nftables rule that permits the specified rule. Example Rules handled by this method:
// nft add rule inet nexodus nexodus-inbound meta nfproto ipv4 ip protocol icmp ip saddr 100.100.0.0/20 counter accept
// nft add rule inet nexodus nexodus-outbound meta nfproto ipv4 ip daddr 100.100.0.1-100.100.0.100 iifname wg0 accept
// nft add rule inet nexodus nexodus-outbound meta nfproto ipv4 ip daddr 8.8.8.8 udp dport 53 iifname "wg0" accept
func (nx *Nexodus) nfPermitProtoPortAddrV4(chain string, rule public.ModelsSecurityRule) error {
	var dportOption, srcOrDst string
	var nft []string

	dportOption = nx.nftPortOption(rule)

	if chain == ingressChain {
		srcOrDst = srcAddr
	} else {
		srcOrDst = destAddr
	}

	switch rule.IpProtocol {
	case protoIPv4:
		// if the specified proto is ipv4 that specifies an L3 address and does not specify ports.
		if rule.FromPort == 0 && rule.ToPort == 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstOption := fmt.Sprintf("ip %s %s", srcOrDst, ipRange)
				// v4 permits for L3 src or dst
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, srcOrDstOption, ruleInterface, counter, actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
	case protoTCP:
		// permit ipv4 tcp to src/dst L3 to any destination port
		if rule.FromPort == 0 && rule.ToPort == 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstOption := fmt.Sprintf("ip %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, srcOrDstOption, protoTCP, destPort, "0-65535", ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
		// permit ipv4 tcp to L3 src/dst to specified destination port or port range
		if rule.FromPort != 0 && rule.ToPort != 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstOption := fmt.Sprintf("ip %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, srcOrDstOption, protoTCP, dportOption, ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
	case protoUDP:
		// permit ipv4 udp to src/dst L3 to any destination port
		if rule.FromPort == 0 && rule.ToPort == 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstOption := fmt.Sprintf("ip %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, srcOrDstOption, protoUDP, destPort, "0-65535", ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
		// permit ipv4 udp to L3 src/dst to specified destination port or port range
		if rule.FromPort != 0 && rule.ToPort != 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstOption := fmt.Sprintf("ip %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, srcOrDstOption, rule.IpProtocol, dportOption, ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
	case protoICMP, protoICMPv4:
		// icmpv4 permits to L3 src or dst
		for _, ipRange := range rule.IpRanges {
			srcOrDstOption := fmt.Sprintf("ip %s %s", srcOrDst, ipRange)
			nft = []string{"insert", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, "ip", "protocol", protoICMP, srcOrDstOption, ruleInterface, counter, actionAccept}
			if _, err := runNftCmd(nx.logger, nft); err != nil {
				return err
			}
		}
	default:
		nx.logger.Debugf("no match for permit proto dport rule: %v", rule)
		return nil
	}

	return nil
}

// nfPermitProtoPortAddrV6 creates a nftables rule that permits the specified rule. Example Rules handled by this method:
// nft add rule inet nexodus nexodus-outbound meta nfproto ipv6 ip6 daddr 2001:4860:4860::8888-2001:4860:4860::8889 udp dport 0-65535 iifname "wg0" accept
// nft add rule inet nexodus nexodus-outbound meta nfproto ipv6 ip6 daddr 2001:4860:4860::8888-2001:4860:4860::8889  iifname "wg0" accept
// nft add rule inet nexodus nexodus-outbound meta nfproto ipv6 ip6 daddr 2001:4860:4860::8888-2001:4860:4860::8889 udp dport 53 iifname "wg0" accept
// nft add rule inet nexodus nexodus-inbound meta nfproto ipv6 ip6 nexthdr ipv6-icmp ip6 saddr 200::/64 counter accept
func (nx *Nexodus) nfPermitProtoPortAddrV6(chain string, rule public.ModelsSecurityRule) error {
	var dportOption, srcOrDst string
	var nft []string

	dportOption = nx.nftPortOption(rule)

	if chain == ingressChain {
		srcOrDst = srcAddr
	} else {
		srcOrDst = destAddr
	}

	switch rule.IpProtocol {
	case protoIPv6:
		// nft add rule inet nexodus nexodus-outbound meta nfproto ipv6 ip6 daddr 2001:4860:4860::8888-2001:4860:4860::8889  iifname "wg0" accept
		// ipv6 that specifies an L3 src/dst and does not specify ports.
		if rule.FromPort == 0 && rule.ToPort == 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstIpAddrOption := fmt.Sprintf("ip6 %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, srcOrDstIpAddrOption, ruleInterface, counter, actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
	case protoTCP:
		// permit ipv4 tcp to src/dst L3 to any destination port
		if rule.FromPort == 0 && rule.ToPort == 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstOption := fmt.Sprintf("ip6 %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, srcOrDstOption, protoTCP, destPort, "0-65535", ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
		// permit ipv6 udp to L3 src/dst to specified destination port or port range
		if rule.FromPort != 0 && rule.ToPort != 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstIpAddrOption := fmt.Sprintf("ip6 %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, srcOrDstIpAddrOption, rule.IpProtocol, dportOption, ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
	case protoUDP:
		// permit ipv4 udp to src/dst L3 to any destination port
		if rule.FromPort == 0 && rule.ToPort == 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstOption := fmt.Sprintf("ip6 %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, srcOrDstOption, protoUDP, destPort, "0-65535", ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
		// permit ipv4 udp to L3 src/dst to specified destination port or port range
		if rule.FromPort != 0 && rule.ToPort != 0 {
			for _, ipRange := range rule.IpRanges {
				srcOrDstIpAddrOption := fmt.Sprintf("ip6 %s %s", srcOrDst, ipRange)
				nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, srcOrDstIpAddrOption, protoUDP, dportOption, ruleInterface, "counter", actionAccept}
				if _, err := runNftCmd(nx.logger, nft); err != nil {
					return err
				}
			}
		}
	case protoICMP, protoICMPv6:
		// icmpv4 permits to L3 src or dst
		for _, ipRange := range rule.IpRanges {
			srcOrDstIpAddrOption := fmt.Sprintf("ip6 %s %s", srcOrDst, ipRange)
			nft = []string{"insert", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, "ip6", "nexthdr", "ipv6-icmp", srcOrDstIpAddrOption, ruleInterface, counter, actionAccept}
			if _, err := runNftCmd(nx.logger, nft); err != nil {
				return err
			}
		}
	default:
		nx.logger.Debugf("no match for permit proto dport rule: %v", rule)
		return nil
	}

	return nil
}

// nfPermitProtoPort creates a nftables rule that permits the specified rule. Example Rules handled by this method:
// nft add rule inet nexodus nexodus-inbound meta nfproto ipv4 iifname "wg0" tcp dport 1-80 counter accept
// nft add rule inet nexodus nexodus-inbound meta nfproto ipv6 iifname "wg0" tcp dport 1-80 counter accept
func (nx *Nexodus) nfPermitProtoPort(chain string, rule public.ModelsSecurityRule) error {
	var dportOption string
	var nft []string
	dportOption = nx.nftPortOption(rule)
	switch rule.IpProtocol {
	case protoIPv4, protoIPv6:
		// if the specified proto is ipv4 or ipv6, add rules for both tcp and udp to the chain with the specified dport
		if dportOption == "" {
			return nil
		}
		// tcp permits for ports to the specified dport for v4/v6
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, protoTCP, dportOption, ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, protoTCP, dportOption, ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
		// udp permits for ports to the specified dport for v4/v6
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, protoUDP, dportOption, ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, protoUDP, dportOption, ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err

		}
	case protoUDP, protoTCP:
		// if the specified proto is tcp or udp, add rules for both ipv4 and ipv6 to the chain with the specified dport
		if dportOption == "" {
			return nil
		}
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, rule.IpProtocol, dportOption, ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, rule.IpProtocol, dportOption, ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
	default:
		nx.logger.Debugf("no match for permit proto dport rule: %v", rule)
		return nil
	}

	return nil
}

// nfPermitProtoAny creates a nftables rule that permits the specified rule. Example Rules handled by this method:
// nft insert rule inet nexodus nexodus-outbound meta nfproto ipv4  iifname "wg0" counter accept
// nft insert rule inet nexodus nexodus-outbound meta nfproto ipv6  iifname "wg0" counter accept
// nft add rule inet nexodus nexodus-inbound meta nfproto ipv4 tcp dport 0-65535 iifname "wg0" counter accept
// nft add rule inet nexodus nexodus-inbound meta nfproto ipv6 tcp dport 0-65535  iifname "wg0" counter accept
func (nx *Nexodus) nfPermitProtoAny(chain string, rule public.ModelsSecurityRule) error {
	var nft []string
	switch rule.IpProtocol {
	case protoIPv4, protoIPv6:
		// permit ipv6 any
		if rule.IpProtocol == protoIPv4 {
			nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", rule.IpProtocol, ruleInterface, counter, actionAccept}
			if _, err := runNftCmd(nx.logger, nft); err != nil {
				return err
			}
		}
		// permit ipv4 any
		if rule.IpProtocol == protoIPv6 {
			nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", rule.IpProtocol, ruleInterface, counter, actionAccept}
			if _, err := runNftCmd(nx.logger, nft); err != nil {
				return err
			}
		}

	case "icmp", protoICMPv4, protoICMPv6:
		// permit icmpv4 any
		if rule.IpProtocol == protoICMPv4 || rule.IpProtocol == "icmp" {
			nft = []string{"insert", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, "ip", "protocol", protoICMP, ruleInterface, counter, actionAccept}
			if _, err := runNftCmd(nx.logger, nft); err != nil {
				return err
			}
		}
		// permit icmpv6 any
		if rule.IpProtocol == protoICMPv6 {
			// ip6 nexthdr is used instead of ip6 protocol for IPv6, because the protocol field is not directly in the IPv6 header.
			nft = []string{"insert", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, "ip6", "nexthdr", "ipv6-icmp", ruleInterface, counter, actionAccept}
			if _, err := runNftCmd(nx.logger, nft); err != nil {
				return err
			}
		}
	case protoTCP, protoUDP:
		// permit ip/ip6 tcp or udp any to all ports
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv4, rule.IpProtocol, destPort, "0-65535", ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
		// permit ipv6 tcp or udp any
		nft = []string{"add", "rule", tableFamily, sgTableName, chain, "meta", "nfproto", protoIPv6, rule.IpProtocol, destPort, "0-65535", ruleInterface, counter, actionAccept}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
	default:
		nx.logger.Debugf("no match for permit proto any dport rule: %v", rule)
		return nil
	}

	return nil
}

// nftPortOption returns the nftables port option for the specified rule.
func (nx *Nexodus) nftPortOption(rule public.ModelsSecurityRule) string {
	var portOption string
	var portRange string

	if rule.FromPort == 0 && rule.ToPort == 0 {
		portRange = fmt.Sprintf("%d-%d", 0, 65535)
	} else if rule.FromPort == rule.ToPort {
		portRange = fmt.Sprintf("%d", rule.FromPort)
	} else {
		portRange = fmt.Sprintf("%d-%d", rule.FromPort, rule.ToPort)
	}
	portOption = fmt.Sprintf("%s %s", destPort, portRange)

	return portOption
}

// nfIngressRuleDrop is used to append a drop rule to the ingress chain. Example rule handled by this method:
func (nx *Nexodus) nfIngressRuleDrop() error {
	nft := []string{"add", "rule", tableFamily, sgTableName, ingressChain, ruleInterface, "counter", actionDrop}
	if _, err := runNftCmd(nx.logger, nft); err != nil {
		return err
	}

	return nil
}

// nfEgressRuleDrop is used to append a drop rule to the egress chain
func (nx *Nexodus) nfEgressRuleDrop() error {
	nft := []string{"add", "rule", tableFamily, sgTableName, egressChain, ruleInterface, "counter", actionDrop}
	if _, err := runNftCmd(nx.logger, nft); err != nil {
		return err
	}

	return nil
}

// nfTableDrop is used to delete the nftables table if it exists
func (nx *Nexodus) nfTableDrop(table string) error {
	// First, check if the table exists
	exists, err := nx.nfTableExists(table)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	// If the table exists, proceed with deletion
	nft := []string{"delete", "table", tableFamily, table}
	if _, err := runNftCmd(nx.logger, nft); err != nil {
		return err
	}

	return nil
}

func (nx *Nexodus) nfTableExists(table string) (bool, error) {
	args := []string{"list", "tables"}
	output, err := runNftCmd(nx.logger, args)
	if err != nil {
		return false, err
	}

	tableFullName := fmt.Sprintf("table %s %s", tableFamily, table)

	// Split the output into lines
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// Trim spaces and check for an exact match
		if strings.TrimSpace(line) == tableFullName {
			return true, nil
		}
	}

	return false, nil
}

// nfCreateTable is used to create the nftables table
func (nx *Nexodus) nfCreateTable(table string) error {
	if _, err := runNftCmd(nx.logger, []string{"add", "table", tableFamily, table}); err != nil {
		return err
	}

	return nil
}

// nfCreateChain is used to create the nftables chain in the nf table
func (nx *Nexodus) nfCreateChain(chainName string) error {
	if _, err := runNftCmd(nx.logger, []string{"add", "chain", tableFamily, sgTableName, chainName, "{", "type", "filter", "hook", "input", "priority", "0", ";", "policy", "accept", ";", "}"}); err != nil {
		return err
	}

	return nil
}

// containsIPv4Range matches the following ipv4 patterns:
// Cidr notation 100.100.0.0/16
// Individual address 10.100.0.2
// Dash-separated range 100.100.0.0-100.100.10.255
func containsIPv4Range(ipRanges []string) bool {
	for _, ipRange := range ipRanges {
		if strings.Contains(ipRange, "-") {
			// Dash-separated range
			ips := strings.Split(ipRange, "-")
			ip1 := net.ParseIP(strings.TrimSpace(ips[0]))
			ip2 := net.ParseIP(strings.TrimSpace(ips[1]))

			if ip1 != nil && ip1.To4() != nil && ip2 != nil && ip2.To4() != nil {
				return true
			}
		} else if strings.Contains(ipRange, "/") {
			// CIDR notation
			_, ipNet, err := net.ParseCIDR(ipRange)
			if err == nil && ipNet.IP.To4() != nil {
				return true
			}
		} else {
			ip := net.ParseIP(ipRange)
			// Individual IP
			if ip != nil && ip.To4() != nil {
				return true
			}
		}
	}

	return false
}

// containsIPv6Range matches the following ipv6 patterns:
// Cidr notation 200::/64
// Individual address 200::2
// Dash-separated range Range 200::1-200::8
// Dash-separated range 2001:0db8:0000:0000:0000:0000:0000:0000-2001:0db8:ffff:ffff:ffff:ffff:ffff:ffff
func containsIPv6Range(ipRanges []string) bool {
	for _, ipRange := range ipRanges {
		if strings.Contains(ipRange, "-") {
			// Dash-separated range
			ips := strings.Split(ipRange, "-")
			if len(ips) != 2 {
				return false
			}

			ip1 := net.ParseIP(strings.TrimSpace(ips[0]))
			ip2 := net.ParseIP(strings.TrimSpace(ips[1]))

			if ip1 == nil || ip2 == nil || ip1.To16() == nil || ip2.To16() == nil {
				return false
			}
		} else if strings.Contains(ipRange, "/") {
			// CIDR notation
			_, _, err := net.ParseCIDR(ipRange)
			if err != nil {
				return false
			}
		} else {
			// Individual IP
			ip := net.ParseIP(ipRange)
			if ip == nil || ip.To16() == nil {
				return false
			}
		}
	}

	return true
}

// runNftCmd is used to execute nft commands
func runNftCmd(logger *zap.SugaredLogger, cmd []string) (string, error) {
	nft := exec.Command("nft", cmd...)

	output, err := nft.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("nft command: nft %q failed: %w\n", strings.Join(cmd, " "), err)
	}
	logger.Debugf("nft command: nft %s", strings.Join(cmd, " "))

	return string(output), nil
}

func debugSecurityGroupRules(logger *zap.SugaredLogger, inboundRules, outboundRules []public.ModelsSecurityRule) error {
	inJson, err := json.MarshalIndent(inboundRules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to print debug json inbound rules: %w", err)
	}
	logger.Debugf("\nInboundRules:\n %s\n", inJson)

	outJson, err := json.MarshalIndent(outboundRules, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to print debug json inbound rules: %w", err)
	}
	logger.Debugf("\nOutboundRules:\n %s\n", outJson)

	return nil
}

// nfNetworkRouterSetup set up the v4/v6 nftables rules for a network router node
func (nx *Nexodus) nfNetworkRouterSetup() error {

	// Drop the existing table if one exists from previous runs
	if err := nx.nfTableDrop(rtrTableName); err != nil {
		return fmt.Errorf("nftables router setup error, failed to flush nftables: %w", err)
	}

	// Create the net router table
	if err := nx.nfCreateTable(rtrTableName); err != nil {
		return fmt.Errorf("nftables setup error, failed to create nftables inet table: %w", err)
	}

	// Create the prerouting nftables chain
	if err := nx.rtrCreateChain(chainPrerouting, chainTypeNAT, priorityDstNAT); err != nil {
		return fmt.Errorf("nftables setup error, failed to create network router nftables chain %s: %w", chainPrerouting, err)
	}

	// Create the postrouting nftables chain
	if err := nx.rtrCreateChain(chainPostrouting, chainTypeNAT, prioritySrcNAT); err != nil {
		return fmt.Errorf("nftables setup error, failed to create network router nftables chain %s: %w", chainTypeNAT, err)
	}

	// Create the forward nftables chain
	if err := nx.rtrCreateChain(chainForward, chainTypeFilter, priorityFilter); err != nil {
		return fmt.Errorf("nftables setup error, failed to create network router nftables chain %s: %w", chainTypeFilter, err)
	}

	// Create the forwarding rule with a prefix and oifname interface for each destination prefix
	for prefix, iface := range nx.netRouterInterfaceMap {
		nx.logger.Debugf("Adding nftables forwarding rule for prefix: %s on interface: %s", prefix, iface.Name)
		nft := []string{"add", "rule", tableFamily, rtrTableName, chainForward, "oifname", iface.Name, "ip", destAddr, prefix, counter, "accept"}
		if _, err := runNftCmd(nx.logger, nft); err != nil {
			return err
		}
	}

	// If --disable-snat was not passed, add a masquerade rule to the postrouting chain,
	if !nx.networkRouterDisableNAT {
		for _, iface := range nx.netRouterInterfaceMap {
			nft := []string{"add", "rule", tableFamily, rtrTableName, chainPostrouting, "oifname", iface.Name, counter, "masquerade"}
			if _, err := runNftCmd(nx.logger, nft); err != nil {
				return err
			}
		}
	}

	return nil
}

// rtrCreateChain is used to create the nftables net-router chain in the nf table
func (nx *Nexodus) rtrCreateChain(chainName, chainType, chainPriority string) error {
	if _, err := runNftCmd(nx.logger, []string{"add", "chain", tableFamily, rtrTableName, chainName, "{", "type", chainType, "hook", chainName, "priority", chainPriority, ";", "}"}); err != nil {
		return err
	}

	return nil
}
