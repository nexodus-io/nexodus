package apex

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/ini.v1"

	log "github.com/sirupsen/logrus"
)

const (
	persistentKeepalive    = "25"
	persistentHubKeepalive = "0"
)

// ParseWireguardConfig parse peerlisting to build the wireguard [Interface] and [Peer] sections
func (ax *Apex) ParseWireguardConfig(listenPort int) {

	var peers []wgPeerConfig
	var localInterface wgLocalConfig
	var hubRouterExists bool

	for _, value := range ax.peerCache {
		var pubkey string
		var ok bool
		if pubkey, ok = ax.keyCache[value.ID]; !ok {
			dest, err := url.JoinPath(ax.controllerURL.String(), fmt.Sprintf(DEVICE_URL, value.DeviceID))
			if err != nil {
				log.Fatalf("unable to create dest url: %s", err)
			}
			req, err := http.NewRequest("GET", dest, nil)
			if err != nil {
				log.Fatalf("cannot create new request: %+v", err)
			}

			token, err := ax.auth.Token()
			if err != nil {
				log.Fatalf("cannot get auth token: %s", err)
			}
			req.Header.Set("authorization", fmt.Sprintf("bearer %s", token))

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Fatalf("cannot send request: %+v", err)
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				log.Fatalf("cannot read body: %+v", err)
			}

			if res.StatusCode != http.StatusOK {
				log.Fatalf("http error: %d %s", res.StatusCode, string(body))
			}

			var device DeviceJSON
			if err := json.Unmarshal(body, &device); err != nil {
				log.Fatalf("cannot unsmarshal device json: %+v", err)
			}
			pubkey = device.PublicKey
			ax.keyCache[value.ID] = device.PublicKey
		}

		if pubkey == ax.wireguardPubKey {
			ax.wireguardPubKeyInConfig = true
		}
		if value.HubRouter {
			hubRouterExists = true
		}
	}
	if !ax.wireguardPubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the controller update\n", ax.wireguardPubKey)
	}
	// determine if the peer listing for this node is a hub zone or hub-router
	for _, value := range ax.peerCache {
		pubkey := ax.keyCache[value.ID]
		if pubkey == ax.wireguardPubKey && value.HubRouter {
			log.Debug("This node is a hub-router")
			if ax.os == Darwin.String() || ax.os == Windows.String() {
				log.Fatalf("Linux nodes are the only supported hub router OS")
			} else {
				// Build a hub-router wireguard configuration
				ax.parseHubWireguardConfig(listenPort)
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
			ax.parseHubWireguardConfig(listenPort)
			return
		}
	}
	// Parse the [Peers] section of the wg config
	for _, value := range ax.peerCache {
		pubkey := ax.keyCache[value.ID]
		// Build the wg config for all peers
		if pubkey != ax.wireguardPubKey {

			var allowedIPs string
			if value.ChildPrefix != "" {
				// check the netlink routing tables for the child prefix and exit if it already exists
				if ax.os == Linux.String() && routeExists(value.ChildPrefix) {
					log.Errorf("unable to add the child-prefix route [ %s ] as it already exists on this linux host", value.ChildPrefix)
				}
				allowedIPs = appendChildPrefix(value.AllowedIPs, value.ChildPrefix)
			} else {
				allowedIPs = value.AllowedIPs
			}
			peer := wgPeerConfig{
				pubkey,
				value.EndpointIP,
				allowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			log.Printf("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
				allowedIPs,
				value.EndpointIP,
				pubkey,
				value.NodeAddress,
				value.ZoneID)
		}
		// Parse the [Interface] section of the wg config
		if pubkey == ax.wireguardPubKey {
			localInterface = wgLocalConfig{
				ax.wireguardPvtKey,
				value.AllowedIPs,
				listenPort,
				false,
				"",
				"",
			}
			log.Printf("Local Node Configuration - Wireguard IP [ %s ] Wireguard Port [ %v ]\n",
				localInterface.Address,
				WgListenPort)
			// set the node unique local interface configuration
			ax.wgConfig.Interface = localInterface
		}
	}
	ax.wgConfig.Peer = peers
}

func (ax *Apex) DeployWireguardConfig() {
	latestCfg := &wgConfig{
		Interface: ax.wgConfig.Interface,
		Peer:      ax.wgConfig.Peer,
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
				if err = applyWireguardConf(); err != nil {
					log.Fatalf("cannot apply wg config: %+v", err)
				}
			} else {
				if err := wgConfPermissions(activeConfig); err != nil {
					log.Errorf("failed to set the wireguard config permissions: %v", err)
				}
				if err = updateWireguardConfig(); err != nil {
					log.Fatalf("cannot update wg config: %+v", err)
				}
			}
		}
		// initiate some traffic to peers, not necessary for p2p only due to PersistentKeepalive
		if ax.hubRouter {
			var wgPeerIPs []string
			for _, peer := range ax.wgConfig.Peer {
				wgPeerIPs = append(wgPeerIPs, peer.AllowedIPs)
			}
			if ax.hubRouterWgIP != "" {
				wgPeerIPs = append(wgPeerIPs, ax.hubRouterWgIP)
			}
			// give the wg0 a second to renegotiate the peering before probing again
			time.Sleep(time.Second * 1)
			probePeers(wgPeerIPs)
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
			// this will throw an error that can be ignored if an existing interface doesn't exist
			wgOut, err := RunCommand("wg-quick", "down", wgIface)
			if err != nil {
				log.Debugf("Failed to down the wireguard interface (this is generally ok): %v\n", err)
			}
			log.Debugf("%v\n", wgOut)
			wgOut, err = RunCommand("wg-quick", "up", activeDarwinConfig)
			if err != nil {
				log.Errorf("failed to start the wireguard interface: %v\n", err)
			}
			log.Debugf("%v", wgOut)
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

func appendChildPrefix(nodeAddress, childPrefix string) string {
	return fmt.Sprintf("%s, %s", nodeAddress, childPrefix)
}
