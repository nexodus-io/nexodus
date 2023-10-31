package nexodus

import (
	"fmt"
	"net"

	"github.com/nexodus-io/nexodus/internal/util"
)

// setupNetworkRouterNode discovers the interface for the network router node and sets up ip forwarding required for the network router node
func (nx *Nexodus) setupNetworkRouterNode() error {
	var err error

	defaultIface, err := findInterfaceForIPRoute(nx.endpointLocalAddress)
	if err != nil {
		return fmt.Errorf("unable to determine default interface for the network router node, please specify using --local-endpoint-ip: %w", err)
	}

	nx.netRouterInterfaceMap = make(map[string]*net.Interface)

	// iterate over childPrefixes and find the best matching interface for each prefix based on the device's
	// default namespace routing table. If no match is found, use the interface containing the default gateway.
	for _, prefix := range nx.childPrefix {
		if util.IsIPv6Prefix(prefix) {
			nx.logger.Warnf("IPv6 is not currently supported for --net-router: %s", prefix)
			continue
		}

		ip, _, err := net.ParseCIDR(prefix)
		if err != nil {
			nx.logger.Errorf("Invalid prefix: %s", prefix)
			continue
		}
		iface, err := findInterfaceForIPRoute(ip.String())
		if err != nil {
			nx.logger.Debugf("No matching interface found for prefix %s, using default interface", prefix)
			iface = defaultIface
		}
		nx.netRouterInterfaceMap[prefix] = iface
	}

	if err := nx.enableForwardingIP(); err != nil {
		return err
	}
	if err := nx.networkRouterSetup(); err != nil {
		return err
	}

	return nil
}
