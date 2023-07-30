package nexodus

import (
	"fmt"

	"go.uber.org/zap"
)

// enableExitSrcValidMarkV4 enables the src_valid_mark functionality for all v4 network interfaces.
func enableExitSrcValidMarkV4() error {
	if _, err := RunCommand("sysctl", "-w", "net.ipv4.conf.all.src_valid_mark=1"); err != nil {
		return fmt.Errorf("failed to enable IPv4 Forwarding for this relay node: %w", err)
	}

	return nil
}

// addExitSrcRuleToRPDB adds a rule to the routing policy database (RPDB) that says, If a packet does
// not have the firewall mark 51820, look up the routing table 51820.
func addExitSrcRuleToRPDB() error {
	if _, err := RunCommand("ip", "-4", "rule", "add", "not", "fwmark", wgFwMark, "table", wgFwMark); err != nil {
		return fmt.Errorf("failed to add fwmark rule to RPDB: %w", err)
	}

	return nil
}

// addExitSrcRuleIgnorePrefixLength adds a rule to the RPDB that says, "When looking up the main routing table, ignore
// the source address prefix length. This is useful for avoiding unnecessary routing cache updates when using policy-based routing.
func addExitSrcRuleIgnorePrefixLength() error {
	if _, err := RunCommand("ip", "-4", "rule", "add", "table", "main", "suppress_prefixlength", "0"); err != nil {
		return fmt.Errorf("failed to add fwmark rule to RPDB: %w", err)
	}

	return nil
}

// addExitSrcDefaultRouteTable adds a default route to the routing table 51820, which says that all traffic should be sent through wg0.
func addExitSrcDefaultRouteTable() error {
	if _, err := RunCommand("ip", "-4", "route", "add", "0.0.0.0/0", "dev", wgIface, "table", wgFwMark); err != nil {
		return fmt.Errorf("failed to add default route to routing table: %w", err)
	}

	return nil
}

// nfAddExitSrcMangleTable create a nftables table for mangle
func nfAddExitSrcMangleTable(logger *zap.SugaredLogger) error {
	if _, err := runNftCmd(logger, []string{"add", "table", "inet", nfOobMangleTable}); err != nil {
		return fmt.Errorf("failed to add nftables table nexodus-stun-mangle: %w", err)
	}

	return nil
}

// nfAddExitSrcMangleOutputChain creates a nftables chain OUTPUT within the nftables mangle (alter) table.
func nfAddExitSrcMangleOutputChain(logger *zap.SugaredLogger) error {
	if _, err := runNftCmd(logger, []string{"add", "chain", "inet", nfOobMangleTable,
		"OUTPUT", "{", "type", "route", "hook", "output", "priority", "mangle", ";", "policy", "accept", ";", "}"}); err != nil {
		return fmt.Errorf("failed to add nftables OUTPUT chain: %w", err)
	}

	return nil
}

// nfAddExitSrcDestPortMangleRule adds a rule to the nftables mangle (alter) table that
// sets the mark 0x4B66 for OOB (out of band) packets sent to a specific port.
func nfAddExitSrcDestPortMangleRule(logger *zap.SugaredLogger, proto string, port int) error {
	if _, err := runNftCmd(logger, []string{"add", "rule", "inet", nfOobMangleTable, "OUTPUT", "meta", "l4proto", proto,
		proto, "dport", fmt.Sprintf("%d", port), "counter", "mark", "set", oobFwdMarkHex}); err != nil {
		return fmt.Errorf("failed to add nftables OUTPUT rule: %w", err)
	}

	return nil
}

// nfAddExitSrcDestPortMangleRule adds a rule to the nftables mangle (alter) table that
// sets the mark 0x4B66 for OOB (out of band) packets sent to a specific port.
func nfAddExitSrcApiServerOOBMangleRule(logger *zap.SugaredLogger, proto, apiServer string, port int) error {
	if _, err := runNftCmd(logger, []string{"add", "rule", "inet", nfOobMangleTable, "OUTPUT", "ip", "daddr", apiServer,
		proto, "dport", fmt.Sprintf("%d", port), "counter", "mark", "set", oobFwdMarkHex}); err != nil {
		return fmt.Errorf("failed to add nftables OUTPUT rule: %w", err)
	}

	return nil
}

// nfAddExitSrcSnatTable create a nftables table for OOB SNAT
func nfAddExitSrcSnatTable(logger *zap.SugaredLogger) error {
	if _, err := runNftCmd(logger, []string{"add", "table", "inet", nfOobSnatTable}); err != nil {
		return fmt.Errorf("failed to add nftables table nexodus-stun-mangle: %w", err)

	}

	return nil
}

// nfAddExitSrcSnatOutputChain the purpose of this chain is to perform source NAT (SNAT) for outgoing packets
func nfAddExitSrcSnatOutputChain(logger *zap.SugaredLogger) error {
	if _, err := runNftCmd(logger, []string{"add", "chain", "inet", nfOobSnatTable, "POSTROUTING", "{", "type", "nat", "hook", "postrouting", "priority", "srcnat", ";", "policy", "accept", ";", "}"}); err != nil {
		return fmt.Errorf("failed to add nftables snat chain POSTROUTING: %w", err)
	}

	return nil
}

// nfAddExitSrcSnatRule adds a rule to the nexodus oob snat table that applies masquerading to postrouting
func nfAddExitSrcSnatRule(logger *zap.SugaredLogger, phyIface string) error {
	if _, err := runNftCmd(logger, []string{"add", "rule", "inet", nfOobSnatTable, "POSTROUTING", "oifname", phyIface, "counter", "masquerade"}); err != nil {
		return fmt.Errorf("failed to add nftables rule SNAT POSTROUTING: %w", err)
	}

	return nil
}

// addExitSrcDefaultRouteTableOOB adds a default route to the OOB routing table, which sources traffic through the physical interface with a gateway
func addExitSrcDefaultRouteTableOOB(phyIface string) error {
	gwIP, err := getDefaultGatewayIPv4()
	if err != nil {
		return fmt.Errorf("failed to find an IPv4 default gateway: %w", err)
	}

	if _, err := RunCommand("ip", "-4", "route", "add", "0.0.0.0/0", "table", oobFwMark, "via", gwIP, "dev", phyIface); err != nil {
		return fmt.Errorf("failed to add default route to routing table %s: %w", oobFwMark, err)
	}

	return nil
}

// addExitSrcRuleFwMarkOOB This command adds a rule to the RPDB that says, If a packet has the firewall mark 19302, look up the routing
// table 19302. This is used to route marked packets with destination port 19302 using the custom routing table
func addExitSrcRuleFwMarkOOB() error {
	if _, err := RunCommand("ip", "-4", "rule", "add", "fwmark", oobFwMark, "table", oobFwMark); err != nil {
		return fmt.Errorf("failed to add OOB fwmark rule to RPDB: %w", err)
	}

	return nil
}

// flushExitSrcRouteTableOOB flushes the specified routing table
func flushExitSrcRouteTableOOB(routeTable string) error {
	if _, err := RunCommand("ip", "route", "flush", "table", routeTable); err != nil {
		return fmt.Errorf("failed to flush routing table %s: %w", routeTable, err)
	}

	return nil
}
