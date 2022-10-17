package aircrew

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/redhat-et/jaywalking/internal/messages"
	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

// parseAircrewControlTowerConfig this is hacky but assumes there is no local config
// or if there is will overwrite it from the publisher peer listing
func (as *AircrewState) ParseAircrewControlTowerConfig(listenPort int, peerListing []messages.Peer) {

	var peers []wgPeerConfig
	var localInterface wgLocalConfig

	for _, value := range peerListing {
		if value.PublicKey == as.NodePubKey {
			as.NodePubKeyInConfig = true
		}
	}

	if !as.NodePubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the control tower update\n", as.NodePubKey)
	}
	// Parse the [Peers] section of the wg config
	for _, value := range peerListing {
		// Build the wg config for all peers
		if value.PublicKey != as.NodePubKey {

			var allowedIPs string
			if value.ChildPrefix != "" {
				allowedIPs = appendChildPrefix(value.AllowedIPs, value.ChildPrefix)
			} else {
				allowedIPs = value.AllowedIPs
			}
			peer := wgPeerConfig{
				value.PublicKey,
				value.EndpointIP,
				allowedIPs,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			log.Printf("Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ] NodeAddress [ %s ] Zone [ %s ]\n",
				allowedIPs,
				value.EndpointIP,
				value.PublicKey,
				value.NodeAddress,
				value.Zone)
		}
		// Parse the [Interface] section of the wg config
		if value.PublicKey == as.NodePubKey {
			localInterface = wgLocalConfig{
				as.NodePvtKey,
				value.AllowedIPs,
				listenPort,
				false,
			}
			log.Printf("Local Node Configuration - Wireguard Local IP [ %s ] Wireguard Port [ %v ]\n",
				localInterface.Address,
				WgListenPort)
			// set the node unique local interface configuration
			as.WgConf.Interface = localInterface
		}
	}
	as.WgConf.Peer = peers
}

func (as *AircrewState) DeployControlTowerWireguardConfig() {
	latestCfg := &wgConfig{
		Interface: as.WgConf.Interface,
		Peer:      as.WgConf.Peer,
	}
	cfg := ini.Empty(ini.LoadOptions{
		AllowNonUniqueSections: true,
	})
	err := ini.ReflectFrom(cfg, latestCfg)
	if err != nil {
		log.Fatal("load ini configuration from struct error")
	}
	switch as.NodeOS {
	case Linux.String():
		latestConfig := filepath.Join(WgLinuxConfPath, wgConfLatestRev)
		if err = cfg.SaveTo(latestConfig); err != nil {
			log.Fatalf("Save latest configuration error: %v\n", err)
		}
		if as.NodePubKeyInConfig {
			// If no config exists, copy the latest config rev to /etc/wireguard/wg0.tomlConf
			activeConfig := filepath.Join(WgLinuxConfPath, wgConfActive)
			if _, err = os.Stat(activeConfig); err != nil {
				if err = applyWireguardConf(); err != nil {
					log.Fatal(err)
				}
			} else {
				if err = updateWireguardConfig(); err != nil {
					log.Fatal(err)
				}
			}
		}
	case Darwin.String():
		activeDarwinConfig := filepath.Join(WgDarwinConfPath, wgConfActive)
		if err = cfg.SaveTo(activeDarwinConfig); err != nil {
			log.Fatalf("Save latest configuration error: %v\n", err)
		}
		if as.NodePubKeyInConfig {
			// this will throw an error that can be ignored if an existing interface doesn't exist
			wgOut, err := RunCommand("wg-quick", "down", wgIface)
			if err != nil {
				log.Debugf("failed to start the wireguard interface: %v\n", err)
			}
			log.Debugf("%v\n", wgOut)
			wgOut, err = RunCommand("wg-quick", "up", activeDarwinConfig)
			if err != nil {
				log.Errorf("failed to start the wireguard interface: %v\n", err)
			}
			log.Debugf("%v", wgOut)
		} else {
			log.Printf("Tunnels not built since the node's public key was found in the configuration")
		}
		log.Printf("Peer setup complete")
	}
}

func appendChildPrefix(nodeAddress, childPrefix string) string {
	allowedIps := fmt.Sprintf("%s, %s", nodeAddress, childPrefix)
	return allowedIps
}
