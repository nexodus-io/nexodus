package nexodus

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const keepaliveInterval = time.Second * 20

// handlePeerTunnel build wg tunnels
func (ax *Nexodus) handlePeerTunnel(wgPeerConfig wgPeerConfig) {
	// validate the endpoint host:port pair parses.
	// temporary: currently if relay state has not converged the endpoint can be registered as (none)
	_, _, err := net.SplitHostPort(wgPeerConfig.Endpoint)
	if err != nil {
		ax.logger.Debugf("failed parse the endpoint address for node [ %s ] (likely still converging) : %v\n", wgPeerConfig.PublicKey, err)
		return
	}

	if err := ax.addPeer(wgPeerConfig); err != nil {
		ax.logger.Errorf("peer tunnel addition failed: %v\n", err)
	}
}

// addPeer add a wg peer
func (ax *Nexodus) addPeer(wgPeerConfig wgPeerConfig) error {
	if ax.userspaceMode {
		return ax.addPeerUS(wgPeerConfig)
	}
	return ax.addPeerOS(wgPeerConfig)
}

// addPeerUs handles adding a new wireguard peer when using the userspace-only mode.
func (ax *Nexodus) addPeerUS(wgPeerConfig wgPeerConfig) error {
	// https://www.wireguard.com/xplatform/#configuration-protocol

	pubDecoded, err := base64.StdEncoding.DecodeString(wgPeerConfig.PublicKey)
	if err != nil {
		ax.logger.Errorf("Failed to decode wireguard public key: %w", err)
		return err
	}

	// Note: The default behavior is "replace_peers=false". If you try to send
	// this, it returns an error. The code only handles "replace_peers=true".
	//config := "replace_peers=false\n"
	config := fmt.Sprintf("public_key=%s\n", hex.EncodeToString(pubDecoded))
	for _, aip := range wgPeerConfig.AllowedIPs {
		config += fmt.Sprintf("allowed_ip=%s\n", aip)
	}
	config += fmt.Sprintf("endpoint=%s\n", wgPeerConfig.Endpoint)
	config += fmt.Sprintf("persistent_keepalive_interval=%d\n", keepaliveInterval/time.Second)

	ax.logger.Debugf("Adding wireguard peer using: %s", config)
	err = ax.userspaceDev.IpcSet(config)
	if err != nil {
		ax.logger.Errorf("Failed to set wireguard config for new peer: %w", err)
		return err
	}
	fullConfig, err := ax.userspaceDev.IpcGet()
	if err != nil {
		ax.logger.Errorf("Failed to read back full wireguard config: %w", err)
		return nil
	}
	ax.logger.Debugf("Updated config: %s", fullConfig)
	return nil
}

// addPeerOS configures a new wireguard peer when using an OS tun networking interface
func (ax *Nexodus) addPeerOS(wgPeerConfig wgPeerConfig) error {
	wgClient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgClient.Close()

	pubKey, err := wgtypes.ParseKey(wgPeerConfig.PublicKey)
	if err != nil {
		return err
	}

	allowedIP := make([]net.IPNet, len(wgPeerConfig.AllowedIPs))
	for i := range wgPeerConfig.AllowedIPs {
		_, ipNet, err := net.ParseCIDR(wgPeerConfig.AllowedIPs[i])
		if err != nil {
			return err
		}
		allowedIP[i] = *ipNet
	}

	LocalIP, endpointPort, err := net.SplitHostPort(wgPeerConfig.Endpoint)
	if err != nil {
		ax.logger.Debugf("failed parse the endpoint address for node [ %s ] (likely still converging) : %v\n", wgPeerConfig.PublicKey, err)
		return err
	}

	port, err := strconv.Atoi(endpointPort)
	if err != nil {
		return err
	}

	udpAddr := &net.UDPAddr{
		IP:   net.ParseIP(LocalIP),
		Port: port,
	}

	keepalive := keepaliveInterval

	// relay nodes do not set explicit endpoints
	cfg := wgtypes.Config{}
	if ax.relay {
		cfg = wgtypes.Config{
			ReplacePeers: false,
			Peers: []wgtypes.PeerConfig{
				{
					PublicKey:                   pubKey,
					Remove:                      false,
					AllowedIPs:                  allowedIP,
					PersistentKeepaliveInterval: &keepalive,
				},
			},
		}
	}
	// all other nodes set peer endpoints
	if !ax.relay {
		cfg = wgtypes.Config{
			ReplacePeers: false,
			Peers: []wgtypes.PeerConfig{
				{
					PublicKey:                   pubKey,
					Remove:                      false,
					Endpoint:                    udpAddr,
					AllowedIPs:                  allowedIP,
					PersistentKeepaliveInterval: &keepalive,
				},
			},
		}
	}

	return wgClient.ConfigureDevice(ax.tunnelIface, cfg)
}

func (ax *Nexodus) handlePeerDelete(peerListing []public.ModelsDevice) error {
	// if the canonical peer listing does not contain a peer from cache, delete the peer
	for _, p := range ax.deviceCache {
		if inPeerListing(peerListing, p.device) {
			continue
		}
		ax.logger.Debugf("Deleting peer with key: %s\n", ax.deviceCache[p.device.PublicKey])
		if err := ax.deletePeer(p.device.PublicKey, ax.tunnelIface); err != nil {
			return fmt.Errorf("failed to delete peer: %w", err)
		}
		// delete the peer route(s)
		ax.handlePeerRouteDelete(ax.tunnelIface, p.device)
		// remove peer from local peer and key cache
		delete(ax.deviceCache, p.device.PublicKey)
	}

	return nil
}

func (ax *Nexodus) deletePeer(publicKey, dev string) error {
	if ax.userspaceMode {
		return ax.deletePeerUS(publicKey)
	} else {
		return ax.deletePeerOS(publicKey, dev)
	}
}

// deletePeerUS deletes a wireguard peer when using a userspace device
func (ax *Nexodus) deletePeerUS(publicKey string) error {
	// https://www.wireguard.com/xplatform/#configuration-protocol

	pubDecoded, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		ax.logger.Errorf("Failed to decode wireguard public key: %w", err)
		return err
	}
	config := fmt.Sprintf("public_key=%s\nremove=true\n", hex.EncodeToString(pubDecoded))
	ax.logger.Debugf("Removing wireguard peer using: %s", config)
	err = ax.userspaceDev.IpcSet(config)
	if err != nil {
		ax.logger.Errorf("Failed to remove wireguard peer (%s): %w", publicKey, err)
		return err
	}
	fullConfig, err := ax.userspaceDev.IpcGet()
	if err != nil {
		ax.logger.Errorf("Failed to read back full wireguard config: %w", err)
		return nil
	}
	ax.logger.Debugf("Updated config: %s", fullConfig)
	return nil
}

// deletePeerOS deletes a wireguard peer when using an OS tun networking device
func (ax *Nexodus) deletePeerOS(publicKey, dev string) error {
	wgClient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgClient.Close()

	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key %s: %w", publicKey, err)
	}

	cfg := []wgtypes.PeerConfig{
		{
			PublicKey: key,
			Remove:    true,
		},
	}

	err = wgClient.ConfigureDevice(dev, wgtypes.Config{
		ReplacePeers: false,
		Peers:        cfg,
	})

	if err != nil {
		return fmt.Errorf("failed to remove peer with key %s: %w", key, err)
	}

	ax.logger.Infof("Removed peer with key %s", key)
	return nil
}

func inPeerListing(peers []public.ModelsDevice, p public.ModelsDevice) bool {
	for _, peer := range peers {
		if peer.Id == p.Id {
			return true
		}
	}
	return false
}

func getWgListenPort() (int, error) {
	l, err := net.ListenUDP("udp", &net.UDPAddr{})
	if err != nil {
		return 0, err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.LocalAddr().String())
	if err != nil {
		return 0, err
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}
	return p, nil
}
