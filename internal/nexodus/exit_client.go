package nexodus

import "fmt"

const (
	wgFwMark         = "51820"
	oobFwMark        = "19302"
	oobFwdMarkHex    = "0x4B66"
	nfOobMangleTable = "nexodus-oob-mangle"
	nfOobSnatTable   = "nexodus-oob-snat"
)

// enableExitSrcValidMarkV4 enables the src_valid_mark functionality for all v4 network interfaces.
func enableExitSrcValidMarkV4() error {
	_, err := RunCommand("sysctl", "-w", "net.ipv4.conf.all.src_valid_mark=1")
	if err != nil {
		return fmt.Errorf("failed to enable IPv4 Forwarding for this relay node: %w", err)
	}

	return nil
}

// addExitSrcRuleToRPDB adds a rule to the routing policy database (RPDB) that says, If a packet does
// not have the firewall mark 51820, look up the routing table 51820.
func addExitSrcRuleToRPDB() error {
	_, err := RunCommand("ip", "-4", "rule", "add", "not", "fwmark", wgFwMark, "table", wgFwMark)
	if err != nil {
		return fmt.Errorf("failed to add fwmark rule to RPDB: %w", err)
	}

	return nil
}

// addExitSrcRuleIgnorePrefixLength adds a rule to the RPDB that says, "When looking up the main routing table, ignore
// the source address prefix length. This is useful for avoiding unnecessary routing cache updates when using policy-based routing.
func addExitSrcRuleIgnorePrefixLength() error {
	_, err := RunCommand("ip", "-4", "rule", "add", "table", "main", "suppress_prefixlength", "0")
	if err != nil {
		return fmt.Errorf("failed to add fwmark rule to RPDB: %w", err)
	}

	return nil
}

// addExitSrcDefaultRouteTable adds a default route to the routing table 51820, which says that all traffic should be sent through wg0.
func addExitSrcDefaultRouteTable() error {
	_, err := RunCommand("ip", "-4", "route", "add", "0.0.0.0/0", "dev", wgIface, "table", wgFwMark)
	if err != nil {
		return fmt.Errorf("failed to add default route to routing table: %w", err)
	}

	return nil
}

// nfAddExitSrcMangleTable create a nftables table for mangle
func nfAddExitSrcMangleTable() error {
	_, err := RunCommand("nft", "add", "table", "ip", nfOobMangleTable)
	if err != nil {
		return fmt.Errorf("failed to add nftables table nexodus-stun-mangle: %w", err)
	}

	return nil
}

// nfAddExitSrcMangleOutputChain creates a nftables chain OUTPUT within the nftables mangle (alter) table.
func nfAddExitSrcMangleOutputChain() error {
	_, err := RunCommand("nft", "add", "chain", "ip", nfOobMangleTable,
		"OUTPUT", "{", "type", "route", "hook", "output", "priority", "mangle", ";", "policy", "accept", ";", "}")
	if err != nil {
		return fmt.Errorf("failed to add nftables chain OUTPUT: %w", err)
	}

	return nil
}

// nfAddExitSrcDestPortMangleRule adds a rule to the nftables mangle (alter) table that
// sets the mark 0x4B66 for OOB (out of band) packets sent to a specific port.
func nfAddExitSrcDestPortMangleRule(proto string, port int) error {
	_, err := RunCommand("nft", "add", "rule", "ip", nfOobMangleTable, "OUTPUT", "meta", "l4proto", proto,
		proto, "dport", fmt.Sprintf("%d", port), "counter", "mark", "set", oobFwdMarkHex)
	if err != nil {
		return fmt.Errorf("failed to add nftables rule OUTPUT: %w", err)
	}

	return nil
}

// nfAddExitSrcDestPortMangleRule adds a rule to the nftables mangle (alter) table that
// sets the mark 0x4B66 for OOB (out of band) packets sent to a specific port.
func nfAddExitSrcApiServerOOBMangleRule(proto, apiServer string, port int) error {
	_, err := RunCommand("nft", "add", "rule", "ip", nfOobMangleTable, "OUTPUT", "ip", "daddr", apiServer,
		proto, "dport", fmt.Sprintf("%d", port), "counter", "mark", "set", oobFwdMarkHex)
	if err != nil {
		return fmt.Errorf("failed to add nftables rule OUTPUT: %w", err)
	}

	return nil
}

// nfAddExitSrcSnatTable create a nftables table for OOB SNAT
func nfAddExitSrcSnatTable() error {
	_, err := RunCommand("nft", "add", "table", "ip", nfOobSnatTable)
	if err != nil {
		return fmt.Errorf("failed to add nftables table nexodus-stun-mangle: %w", err)
	}

	return nil
}

// purpose of this chain is to perform source NAT (SNAT) for outgoing packets
func nfAddExitSrcSnatOutputChain() error {
	_, err := RunCommand("nft", "add", "chain", "ip", nfOobSnatTable, "POSTROUTING", "{", "type", "nat", "hook", "postrouting", "priority", "srcnat", ";", "policy", "accept", ";", "}")
	if err != nil {
		return fmt.Errorf("failed to add nftables snat chain POSTROUTING: %w", err)
	}

	return nil
}

// nfAddExitSrcSnatRule adds a rule to the nexodus oob snat table that applies masquerading to postrouting
func nfAddExitSrcSnatRule(phyIface string) error {
	_, err := RunCommand("nft", "add", "rule", "ip", nfOobSnatTable, "POSTROUTING", "oifname", phyIface, "counter", "masquerade")
	if err != nil {
		return fmt.Errorf("failed to add nftables rule POSTROUTING: %w", err)
	}

	return nil
}

// addExitSrcDefaultRouteTableOOB adds a default route to the OOB routing table, which sources traffic through the physical interface with a gateway
func addExitSrcDefaultRouteTableOOB(phyIface string) error {
	_, err := RunCommand("ip", "-4", "route", "add", "0.0.0.0/0", "table", oobFwMark, "via", "192.168.64.1", "dev", "enp0s1")
	if err != nil {
		return fmt.Errorf("failed to add default route to routing table %s: %w", oobFwMark, err)
	}

	return nil
}

// This command adds a rule to the RPDB that says, If a packet has the firewall mark 19302, look up the routing
// table 19302. This is used to route marked packets with destination port 19302 using the custom routing table
func addExitSrcRuleFwMarkOOB() error {
	_, err := RunCommand("ip", "-4", "rule", "add", "fwmark", oobFwMark, "table", oobFwMark)
	if err != nil {
		return fmt.Errorf("failed to add OOB fwmark rule to RPDB: %w", err)
	}

	return nil
}

// flushExitSrcRouteTableOOB flushes the specified routing table
func flushExitSrcRouteTableOOB(routeTable string) error {
	_, err := RunCommand("ip", "route", "flush", "table", routeTable)
	if err != nil {
		return fmt.Errorf("failed to flush routing table %s: %w", routeTable, err)
	}

	return nil
}

// deleteExitSrcTable remove the origin/server node nft table
func deleteExitSrcTable(nfTable string) error {
	_, err := RunCommand("nft", "delete", "table", "ip", nfTable)
	if err != nil {
		return fmt.Errorf("failed to delete nftables table %s: %w", nfTable, err)
	}

	return nil
}
