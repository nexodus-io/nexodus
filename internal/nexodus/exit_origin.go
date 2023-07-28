package nexodus

import "fmt"

// Origin netfilter and forwarding configuration
// sysctl -w net.ipv4.ip_forward=1
// nft add table inet nexodus-exit-node
// nft add chain inet nexodus-exit-node prerouting '{ type nat hook prerouting priority dstnat; }'
// nft add chain inet nexodus-exit-node postrouting '{ type nat hook postrouting priority srcnat; }'
// nft add chain inet nexodus-exit-node forward '{ type filter hook forward priority filter; }'
// nft add rule inet nexodus-exit-node postrouting oifname "<PHYSICAL_IFACE>" counter masquerade
// nft add rule inet nexodus-exit-node forward iifname "wg0" counter accept

const (
	nfExitNodeTable = "nexodus-exit-node"
)

func addExitDestinationTable() error {
	_, err := RunCommand("nft", "add", "table", "ip", nfExitNodeTable)
	if err != nil {
		return fmt.Errorf("failed to add nftables table %s: %w", nfExitNodeTable, err)
	}

	return nil
}

func addExitOriginPreroutingChain() error {
	_, err := RunCommand("nft", "add", "chain", "ip", nfExitNodeTable, "prerouting", "{", "type", "nat", "hook", "prerouting", "priority", "dstnat", "}")
	if err != nil {
		return fmt.Errorf("failed to add nftables chain nexodus-exit-node: %w", err)
	}
	return nil
}

func addExitOriginPostroutingChain() error {
	_, err := RunCommand("nft", "add", "chain", "ip", nfExitNodeTable, "postrouting", "{", "type", "nat", "hook", "postrouting", "priority", "srcnat", "}")
	if err != nil {
		return fmt.Errorf("failed to add nftables chain nexodus-exit-node: %w", err)
	}

	return nil
}

func addExitOriginForwardChain() error {
	_, err := RunCommand("nft", "add", "chain", "ip", nfExitNodeTable, "forward", "{", "type", "filter", "hook", "forward", "priority", "filter", "}")
	if err != nil {
		return fmt.Errorf("failed to add nftables chain nexodus-exit-node: %w", err)
	}

	return nil
}

func addExitOriginPostroutingRule(phyIface string) error {
	_, err := RunCommand("nft", "add", "rule", "ip", nfExitNodeTable, "postrouting", "oifname", phyIface, "masquerade")
	if err != nil {
		return fmt.Errorf("failed to add nftables rule nexodus-exit-node: %w", err)
	}

	return nil
}

// sudo nft add rule inet nexodus-exit-node forward iifname "wg0" accept
func addExitOriginForwardRule() error {
	_, err := RunCommand("nft", "add", "rule", "ip", nfExitNodeTable, "forward", "iifname", wgIface, "accept")
	if err != nil {
		return fmt.Errorf("failed to add nftables rule nexodus-exit-node: %w", err)
	}

	return nil
}
