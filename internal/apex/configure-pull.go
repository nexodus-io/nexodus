package apex

import (
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

const (
	// wg keepalives are disabled and managed by the agent
	persistentKeepalive    = "0"
	persistentHubKeepalive = "0"
)

// ParseWireguardConfig parse peerlisting to build the wireguard [Interface] and [Peer] sections
func (ax *Apex) ParseWireguardConfig() {

	var peers []wgPeerConfig
	var localInterface wgLocalConfig
	var hubRouterExists bool

	for _, peer := range ax.peerCache {
		var pubkey string
		var ok bool
		if pubkey, ok = ax.keyCache[peer.DeviceID]; !ok {
			device, err := ax.client.GetDevice(peer.DeviceID)
			if err != nil {
				log.Fatalf("unable to get device %s: %s", peer.DeviceID, err)
			}
			ax.keyCache[peer.DeviceID] = device.PublicKey
			pubkey = device.PublicKey
		}

		if pubkey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}
		if peer.HubRouter {
			hubRouterExists = true
		}
	}
	if !ax.wireguardPubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the controller update\n", ax.wireguardPubKey)
	}
	// determine if the peer listing for this node is a hub zone or hub-router
	for _, value := range ax.peerCache {
		pubkey := ax.keyCache[value.DeviceID]
		if pubkey == ax.wireguardPubKey && value.HubRouter {
			log.Debug("This node is a hub-router")
			if ax.os == Darwin.String() || ax.os == Windows.String() {
				log.Fatalf("Linux nodes are the only supported hub router OS")
			} else {
				// Build a hub-router wireguard configuration
				ax.parseHubWireguardConfig(ax.listenPort)
				return
			}
		}
		if value.HubZone {
			log.Debug("This zone is a hub-zone")
			if !hubRouterExists {
				log.Error("cannot deploy to a hub-zone if no hub router has joined the zone yet. See `--hub-router`")
				os.Exit(1)
			}
			// build a hub-zone wireguard configuration
			ax.parseHubWireguardConfig(ax.listenPort)
			return
		}
	}
	// Parse the [Peers] section of the wg config
	for _, value := range ax.peerCache {
		pubkey := ax.keyCache[value.DeviceID]
		// Build the wg config for all peers
		if pubkey != ax.wireguardPubKey {
			if value.ChildPrefix != "" {
				// check the netlink routing tables for the child prefix and exit if it already exists
				if ax.os == Linux.String() && routeExists(value.ChildPrefix) {
					log.Errorf("unable to add the child-prefix route [ %s ] as it already exists on this linux host", value.ChildPrefix)
				} else {
					if ax.os == Linux.String() {
						if err := addLinuxChildPrefixRoute(value.ChildPrefix); err != nil {
							log.Infof("error adding the child prefix route: %v", err)
						}
					}
				}
				// add osx child prefix
				if ax.os == Darwin.String() {
					if err := addDarwinChildPrefixRoute(value.ChildPrefix); err != nil {
						log.Infof("error adding the child prefix route: %v", err)
					}
				}
				value.AllowedIPs = append(value.AllowedIPs, value.ChildPrefix)
			}
			peer := wgPeerConfig{
				pubkey,
				value.EndpointIP,
				value.AllowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			log.Printf("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
				value.AllowedIPs,
				value.EndpointIP,
				pubkey,
				value.NodeAddress,
				value.ZoneID)
		}
		// check if the controller has assigned a new address
		if pubkey == ax.wireguardPubKey {
			// replace the interface with the newly assigned interface
			if ax.wgLocalAddress != value.NodeAddress {
				log.Infof("New local interface address assigned %s", value.NodeAddress)
				if ax.os == Linux.String() && linkExists(wgIface) {
					if err := delLink(wgIface); err != nil {
						// not a fatal error since if this is on startup it could be absent
						log.Debugf("failed to delete netlink interface %s: %v", wgIface, err)
					}
				}
				if ax.os == Darwin.String() {
					if ifaceExists(darwinIface) {
						deleteDarwinIface()
					}
				}
			}
			ax.wgLocalAddress = value.NodeAddress
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				ax.listenPort,
			}
			log.Printf("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ]\n",
				ax.wgLocalAddress,
				ax.listenPort)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
	}
	ax.wgConfig.Peers = peers
}

func (ax *Apex) DeployWireguardConfig(newPeers []models.Peer, firstTime bool) {
	latestCfg := &wgConfig{
		Interface: ax.wgConfig.Interface,
		Peers:     ax.wgConfig.Peers,
	}
	cfg := ini.Empty(ini.LoadOptions{
		AllowNonUniqueSections: true,
	})
	err := ini.ReflectFrom(cfg, latestCfg)
	if err != nil {
		log.Fatal("load ini configuration from struct error")
	}
	switch ax.os {
	case Linux.String():
		latestConfig := filepath.Join(WgLinuxConfPath, wgConfLatestRev)
		if err = cfg.SaveTo(latestConfig); err != nil {
			log.Fatalf("save latest configuration error: %v\n", err)
		}
		if err := wgConfPermissions(latestConfig); err != nil {
			log.Errorf("failed to set the wireguard config permissions: %v", err)
		}
		if ax.wireguardPubKeyInConfig {
			// If no config exists, copy the latest config rev to /etc/wireguard/wg0.tomlConf
			activeConfig := filepath.Join(WgLinuxConfPath, wgConfActive)
			if _, err = os.Stat(activeConfig); err != nil {
				if err = ax.overwriteWgConfig(); err != nil {
					log.Fatalf("cannot apply wg config: %+v", err)
				}
			} else {
				if err := wgConfPermissions(activeConfig); err != nil {
					log.Errorf("failed to set the wireguard config permissions: %v", err)
				}
				if err = ax.updateWireguardConfig(); err != nil {
					log.Fatalf("cannot update wg config: %+v", err)
				}
			}
		}
		// initialize the wireguard interface if it does not have an address
		if ax.wgLocalAddress != getIPv4Iface(wgIface).String() {
			ax.setupLinuxInterface()
		}

		// add routes and tunnels for all peer candidates without checking cache since it has not been built yet
		if firstTime {
			for _, peer := range latestCfg.Peers {
				ax.handlePeerRoute(peer)
				ax.handlePeerTunnel(peer)
			}
		}
		// add routes and tunnels for the new peers only according to the cache diff
		for _, newPeer := range newPeers {
			if newPeer.ID != uuid.Nil {
				device, err := ax.client.GetDevice(newPeer.DeviceID)
				if err != nil {
					log.Errorf("unable to get device %s: %s", newPeer.DeviceID, err)
				}
				// add routes for each peer candidate (unless the key matches the local nodes key)
				for _, peer := range latestCfg.Peers {
					if peer.PublicKey == device.PublicKey && device.PublicKey != ax.wireguardPubKey {
						ax.handlePeerRoute(peer)
						ax.handlePeerTunnel(peer)
					}
				}
			}
		}

	case Darwin.String():
		activeDarwinConfig := filepath.Join(WgDarwinConfPath, wgConfActive)
		if err = cfg.SaveTo(activeDarwinConfig); err != nil {
			log.Fatalf("save latest configuration error: %v\n", err)
		}
		if err := wgConfPermissions(activeDarwinConfig); err != nil {
			log.Errorf("failed to set the wireguard config permissions: %v", err)
		}
		if ax.wireguardPubKeyInConfig {
			if err := setupDarwinIface(ax.wgLocalAddress); err != nil {
				log.Tracef("%v", err)
			}
		}
		// add routes for each peer candidate
		for _, peer := range latestCfg.Peers {
			ax.handlePeerRoute(peer)
		}
		// add tunnels for each peer candidate
		for _, peer := range latestCfg.Peers {
			ax.handlePeerTunnel(peer)
		}

	case Windows.String():
		activeWindowsConfig := filepath.Join(WgWindowsConfPath, wgConfActive)
		if err = cfg.SaveTo(activeWindowsConfig); err != nil {
			log.Fatalf("save latest configuration error: %v\n", err)
		}
		if ax.wireguardPubKeyInConfig {
			// this will throw an error that can be ignored if an existing interface doesn't exist
			wgOut, err := RunCommand("wireguard.exe", "/uninstalltunnelservice", wgIface)
			if err != nil {
				log.Debugf("Failed to down the wireguard interface (this is generally ok): %v\n", err)
			}
			log.Debugf("%v\n", wgOut)
			// sleep for one second to give the wg async exe time to tear down any existing wg0 configuration
			time.Sleep(time.Second * 1)
			// windows implementation does not handle certain fields the osx and linux wg configs can
			sanitizeWindowsConfig(activeWindowsConfig)
			wgOut, err = RunCommand("wireguard.exe", "/installtunnelservice", activeWindowsConfig)
			if err != nil {
				log.Errorf("failed to start the wireguard interface: %v\n", err)
			}
			log.Debugf("%v\n", wgOut)
		}
	}
	log.Printf("Peer setup complete")
}

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
// TODO: routes need to be looked up if the exists, netlink etc.
func (ax *Apex) handlePeerRoute(wgPeerConfig wgPeerConfig) {
	switch ax.os {
	case Darwin.String():
		// Darwin maps to a tunX address which needs to be discovered
		netName, err := getInterfaceByIP(net.ParseIP(ax.wgLocalAddress))
		if err != nil {
			log.Debugf("failed to find the darwin interface with the address [ %s ] %v", ax.wgLocalAddress, err)
		}
		// If child prefix split the two prefixes (host /32 and child prefix
		for _, allowedIP := range wgPeerConfig.AllowedIPs {
			_, err := RunCommand("route", "-q", "-n", "delete", "-inet", allowedIP, "-interface", netName)
			if err != nil {
				log.Tracef("no route deleted: %v", err)
			}
			_, err = RunCommand("route", "-q", "-n", "add", "-inet", allowedIP, "-interface", netName)
			if err != nil {
				log.Tracef("child prefix route add failed: %v", err)
			}
		}
	case Linux.String():
		for _, allowedIP := range wgPeerConfig.AllowedIPs {
			_, err := RunCommand("ip", "route", "del", allowedIP, "dev", wgIface)
			if err != nil {
				log.Tracef("no route deleted: %v", err)
			}
			_, err = RunCommand("ip", "route", "add", allowedIP, "dev", wgIface)
			if err != nil {
				log.Tracef("route add failed: %v", err)
			}
		}
	}
}

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
// TODO: routes need to be looked up if the exists, netlink etc.
func (ax *Apex) handlePeerTunnel(wgPeerConfig wgPeerConfig) {
	// validate the endpoint host:port pair parses.
	// temporary: currently if relay state has not converged the endpoint can be registered as (none)
	_, _, err := net.SplitHostPort(wgPeerConfig.Endpoint)
	if err != nil {
		log.Debugf("failed parse the endpoint address for node [ %s ] (likely still converging) : %v\n", wgPeerConfig.PublicKey, err)
		return
	}
	allowedIPs := allowedIPsString(wgPeerConfig.AllowedIPs)
	switch ax.os {
	case Darwin.String():
		// remove a prior entry for the peer (fails silently)
		_, err := RunCommand("wg", "set", darwinIface, "peer", wgPeerConfig.PublicKey, "remove")
		if err != nil {
			log.Errorf("peer tunnel removal failed: %v", err)
		}
		// insert the peer
		_, err = RunCommand("wg", "set", darwinIface, "peer", wgPeerConfig.PublicKey, "allowed-ips", allowedIPs, "endpoint", wgPeerConfig.Endpoint)
		if err != nil {
			log.Errorf("peer tunnel addition failed: %v", err)
		}
	case Linux.String():
		// remove a prior entry for the peer
		_, err := RunCommand("wg", "set", wgIface, "peer", wgPeerConfig.PublicKey, "remove")
		if err != nil {
			log.Errorf("peer tunnel removal failed: %v", err)
		}
		// bouncers to not get a persistent keepalive or specific endpoints since
		// the source ports from peers need to be dynamic for symmetric NAT.
		if ax.hubRouter {
			_, err = RunCommand("wg", "set", wgIface, "peer", wgPeerConfig.PublicKey, "allowed-ips", allowedIPs)
			if err != nil {
				log.Errorf("peer tunnel addition failed: %v", err)
			}
		} else {
			_, err = RunCommand("wg", "set", wgIface, "peer", wgPeerConfig.PublicKey, "allowed-ips", allowedIPs, "endpoint", wgPeerConfig.Endpoint)
			if err != nil {
				log.Errorf("peer tunnel addition failed: %v", err)
			}
		}
	}
}

// allowedIPsString joins the allowedIPs slice into the format required for wg set peer x,y,z
func allowedIPsString(ips []string) string {
	return strings.Join(ips, ",")
}

func getWgListenPort() int {
	min := 32768
	max := 61000
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}
