package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

type PeerListing []Peer

// Peer REST struct
type Peer struct {
	PublicKey   string `json:"PublicKey"`
	EndpointIP  string `json:"EndpointIP"`
	AllowedIPs  string `json:"AllowedIPs"`
	Zone        string `json:"Zone"`
	NodeAddress string `json:"NodeAddress"`
	ChildPrefix string `json:"ChildPrefix"`
}

// handleMsg deal with streaming messages
func handleMsg(payload string) PeerListing {
	var peerListing PeerListing
	err := json.Unmarshal([]byte(payload), &peerListing)
	if err != nil {
		log.Debugf("Unmarshalling error from handleMsg: %v\n", err)
		return nil
	}
	return peerListing
}

// parseAircrewControlTowerConfig this is hacky but assumes there is no local config
// or if there is will overwrite it from the publisher peer listing
func (as *aircrewState) parseAircrewControlTowerConfig(peerListing PeerListing) {

	var peers []wgPeerConfig
	var localInterface wgLocalConfig

	for _, value := range peerListing {
		if value.PublicKey == as.nodePubKey {
			as.nodePubKeyInConfig = true
		}
	}

	if !as.nodePubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the control tower update\n", as.nodePubKey)
	}
	// Parse the [Peers] section of the wg config
	for _, value := range peerListing {
		// Build the wg config for all peers
		if value.PublicKey != as.nodePubKey {

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
		if value.PublicKey == as.nodePubKey {
			localInterface = wgLocalConfig{
				as.nodePvtKey,
				value.AllowedIPs,
				cliFlags.listenPort,
				false,
			}
			log.Printf("Local Node Configuration - Wireguard Local IP [ %s ] Wireguard Port [ %v ]\n",
				localInterface.Address,
				wgListenPort)
			// set the node unique local interface configuration
			as.wgConf.Interface = localInterface
		}
	}
	as.wgConf.Peer = peers
}

func (as *aircrewState) deployControlTowerWireguardConfig() {
	latestCfg := &wgConfig{
		Interface: as.wgConf.Interface,
		Peer:      as.wgConf.Peer,
	}
	cfg := ini.Empty(ini.LoadOptions{
		AllowNonUniqueSections: true,
	})
	err := ini.ReflectFrom(cfg, latestCfg)
	if err != nil {
		log.Fatal("load ini configuration from struct error")
	}
	switch as.nodeOS {
	case Linux.String():
		latestConfig := filepath.Join(wgLinuxConfPath, wgConfLatestRev)
		if err = cfg.SaveTo(latestConfig); err != nil {
			log.Fatalf("Save latest configuration error: %v\n", err)
		}
		if as.nodePubKeyInConfig {
			// If no config exists, copy the latest config rev to /etc/wireguard/wg0.tomlConf
			activeConfig := filepath.Join(wgLinuxConfPath, wgConfActive)
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
		activeDarwinConfig := filepath.Join(wgDarwinConfPath, wgConfActive)
		if err = cfg.SaveTo(activeDarwinConfig); err != nil {
			log.Fatalf("Save latest configuration error: %v\n", err)
		}
		if as.nodePubKeyInConfig {
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
