package nexodus

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/client"
	"net"
	"strconv"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const keepaliveInterval = time.Second * 20

// handlePeerTunnel build wg tunnels
func (nx *Nexodus) handlePeerTunnel(wgPeerConfig wgPeerConfig) error {
	// validate the endpoint host:port pair parses.
	// temporary: currently if relay state has not converged the endpoint can be registered as (none)
	_, _, err := net.SplitHostPort(wgPeerConfig.Endpoint)
	if err != nil {
		nx.logger.Debugf("failed parse the endpoint address for node [ %s ] (likely still converging) : %v\n", wgPeerConfig.PublicKey, err)
		return nil
	}

	if err := nx.addPeer(wgPeerConfig); err != nil {
		nx.logger.Errorf("peer tunnel addition failed: %v\n", err)
		return err
	}

	return nil
}

// addPeer add a wg peer
func (nx *Nexodus) addPeer(wgPeerConfig wgPeerConfig) error {
	if nx.userspaceMode {
		return nx.addPeerUS(wgPeerConfig)
	}
	return nx.addPeerOS(wgPeerConfig)
}

// addPeerUs handles adding a new wireguard peer when using the userspace-only mode.
func (nx *Nexodus) addPeerUS(wgPeerConfig wgPeerConfig) error {
	// https://www.wireguard.com/xplatform/#configuration-protocol

	pubDecoded, err := base64.StdEncoding.DecodeString(wgPeerConfig.PublicKey)
	if err != nil {
		nx.logger.Errorf("Failed to decode wireguard public key: %w", err)
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

	nx.logger.Debugf("Adding wireguard peer using: %s", config)
	err = nx.userspaceDev.IpcSet(config)
	if err != nil {
		nx.logger.Errorf("Failed to set wireguard config for new peer: %w", err)
		return err
	}
	fullConfig, err := nx.userspaceDev.IpcGet()
	if err != nil {
		nx.logger.Errorf("Failed to read back full wireguard config: %w", err)
		return nil
	}
	nx.logger.Debugf("Updated config: %s", fullConfig)
	return nil
}

// addPeerOS configures a new wireguard peer when using an OS tun networking interface
func (nx *Nexodus) addPeerOS(wgPeerConfig wgPeerConfig) error {
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
		nx.logger.Debugf("failed parse the endpoint address for node [ %s ] (likely still converging) : %v\n", wgPeerConfig.PublicKey, err)
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
	if nx.relay {
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
	if !nx.relay {
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

	return wgClient.ConfigureDevice(nx.tunnelIface, cfg)
}

// assumes a write lock is held on deviceCacheLock
func (nx *Nexodus) handlePeerDelete(peerMap map[string]client.ModelsDevice) error {
	// if the canonical peer listing does not contain a peer from cache, delete the peer
	for _, p := range nx.deviceCache {
		if _, ok := peerMap[p.device.GetId()]; ok {
			continue
		}

		if err := nx.peerCleanup(p.device); err != nil {
			return err
		}
		// remove peer from local peer and key cache
		delete(nx.deviceCache, p.device.GetPublicKey())
	}

	return nil
}

func (nx *Nexodus) peerCleanup(peer client.ModelsDevice) error {
	nx.logger.Debugf("Deleting peering config for key: %s\n", peer.GetPublicKey())
	if err := nx.deletePeer(peer.GetPublicKey(), nx.tunnelIface); err != nil {
		return fmt.Errorf("failed to delete peer: %w", err)
	}
	// delete the peer route(s)
	nx.handlePeerRouteDelete(nx.tunnelIface, peer)

	return nil
}

func (nx *Nexodus) deletePeer(publicKey, dev string) error {
	if nx.userspaceMode {
		return nx.deletePeerUS(publicKey)
	} else {
		return nx.deletePeerOS(publicKey, dev)
	}
}

// deletePeerUS deletes a wireguard peer when using a userspace device
func (nx *Nexodus) deletePeerUS(publicKey string) error {
	// https://www.wireguard.com/xplatform/#configuration-protocol

	pubDecoded, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		nx.logger.Errorf("Failed to decode wireguard public key: %w", err)
		return err
	}
	config := fmt.Sprintf("public_key=%s\nremove=true\n", hex.EncodeToString(pubDecoded))
	nx.logger.Debugf("Removing wireguard peer using: %s", config)
	err = nx.userspaceDev.IpcSet(config)
	if err != nil {
		nx.logger.Errorf("Failed to remove wireguard peer (%s): %w", publicKey, err)
		return err
	}
	fullConfig, err := nx.userspaceDev.IpcGet()
	if err != nil {
		nx.logger.Errorf("Failed to read back full wireguard config: %w", err)
		return nil
	}
	nx.logger.Debugf("Updated config: %s", fullConfig)
	return nil
}

// deletePeerOS deletes a wireguard peer when using an OS tun networking device
func (nx *Nexodus) deletePeerOS(publicKey, dev string) error {
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

	nx.logger.Infof("Removed peer with key %s", key)
	return nil
}

func testWgListenPort(port int) error {
	l, err := net.ListenUDP("udp", &net.UDPAddr{Port: port})
	if err != nil {
		return err
	}
	l.Close()
	return nil
}

// getWgListenPort() will allocate a random UDP port to use as our wireguard listen port
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
