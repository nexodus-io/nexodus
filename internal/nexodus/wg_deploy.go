package nexodus

import (
	"errors"
	"fmt"
	"net"

	"github.com/nexodus-io/nexodus/internal/api/public"
)

const (
	persistentKeepalive = "20"
)

var (
	securityGroupErr = errors.New("nftables setup error")
)

func (nx *Nexodus) DeployWireguardConfig(updatedPeers map[string]public.ModelsDevice) error {
	cfg := &wgConfig{
		Interface: nx.wgConfig.Interface,
		Peers:     nx.wgConfig.Peers,
	}

	if nx.TunnelIP != nx.getIPv4Iface(nx.tunnelIface).String() {
		if err := nx.setupInterface(); err != nil {
			return err
		}
	}

	// keep track of the last error that occured during config setup which can be returned at the end
	var lastErr error
	// add routes and tunnels for the new peers only according to the cache diff
	for _, updatedPeer := range updatedPeers {
		if updatedPeer.Id == "" {
			continue
		}
		// add routes for each peer candidate (unless the key matches the local nodes key)
		peer, ok := cfg.Peers[updatedPeer.PublicKey]
		if !ok || peer.PublicKey == nx.wireguardPubKey {
			continue
		}
		if err := nx.handlePeerRoute(peer); err != nil {
			nx.logger.Errorf("Failed to handle peer route: %v", err)
			lastErr = err
		}
		if err := nx.handlePeerTunnel(peer); err != nil {
			nx.logger.Errorf("Failed to handle peer tunnel: %v", err)
			lastErr = err
		}
	}

	nx.logger.Debug("Peer setup complete")
	return lastErr
}

func (nx *Nexodus) setupInterface() error {
	if nx.userspaceMode {
		return nx.setupInterfaceUS()
	}

	// Determine if nx.TunnelIP or nx.TunnelIpV6 overlaps with any of the system interfaces
	// If so, return an error
	if err := checkIPConflict(nx.TunnelIP); err != nil {
		return err
	}
	if err := checkIPConflict(nx.TunnelIpV6); err != nil {
		return err
	}

	return nx.setupInterfaceOS()
}

func checkIPConflict(ip string) error {
	// Parse the IP string to net.IP
	ipNet := net.ParseIP(ip)
	if ipNet == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	// Get all network interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}

	// Check if the IP is in any subnet
	for _, addr := range addrs {
		_, subnet, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}

		if subnet.Contains(ipNet) {
			return fmt.Errorf("IP address %s conflicts with subnet %s", ip, subnet)
		}
	}

	return nil
}
