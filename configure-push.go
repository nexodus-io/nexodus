package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

type PeerListing []Peer

// Peer REST struct
type Peer struct {
	PublicKey  string `json:"PublicKey"`
	EndpointIP string `json:"EndpointIP"`
	AllowedIPs string `json:"AllowedIPs"`
	Zone       string `json:"Zone"`
}

// handleMsg deal with streaming messages
func handleMsg(payload string) PeerListing {
	var peerListing PeerListing
	err := json.Unmarshal([]byte(payload), &peerListing)
	if err != nil {
		log.Printf("[ERROR] UnmarshalMessage: %v\n", err)
		return nil
	}
	return peerListing
}

// parseJaywalkSupervisorConfig this is hacky but assumes there is no local config
// or if there is will overwrite it from the publisher peer listing
func (js *jaywalkState) parseJaywalkSupervisorConfig(peerListing PeerListing) {

	var peers []wgPeerConfig
	var localInterface wgLocalConfig

	for _, value := range peerListing {
		if value.PublicKey == js.nodePubKey {
			js.nodePubKeyInConfig = true
		}
	}

	if !js.nodePubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the broker update", js.nodePubKey)
	}

	for _, value := range peerListing {
		if value.PublicKey != js.nodePubKey {
			peer := wgPeerConfig{
				value.PublicKey,
				value.EndpointIP,
				value.AllowedIPs,
			}
			peers = append(peers, peer)
			log.Printf("[DEBUG] Peer Node Configuration - Peer AllowedIPs [ %s ] Peer Endpoint IP [ %s ] Peer Public Key [ %s ]\n",
				value.AllowedIPs,
				value.EndpointIP,
				value.PublicKey)
		}
		if value.PublicKey == js.nodePubKey {
			localInterface = wgLocalConfig{
				cliFlags.wireguardPvtKey,
				value.AllowedIPs,
				cliFlags.listenPort,
				false,
			}
			log.Printf("[DEBUG] Local Node Configuration - Wireguard Local Endpoint IP [ %s ] Port [ %v ] Local Private Key [ %s ]\n",
				localInterface.Address,
				wgListenPort,
				localInterface.PrivateKey)
			// set the node unique local interface configuration
			js.wgConf.Interface = localInterface
		}
	}
	js.wgConf.Peer = peers
}

func (js *jaywalkState) deploySupervisorWireguardConfig() {
	latestCfg := &wgConfig{
		Interface: js.wgConf.Interface,
		Peer:      js.wgConf.Peer,
	}
	cfg := ini.Empty(ini.LoadOptions{
		AllowNonUniqueSections: true,
	})
	err := ini.ReflectFrom(cfg, latestCfg)
	if err != nil {
		log.Fatal("load ini configuration from struct error")
	}
	switch js.nodeOS {
	case linux.String():
		// wg does not create the OSX config directory by default
		if err = createDirectory(wgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", wgDarwinConfPath, err)
		}
		latestConfig := filepath.Join(wgLinuxConfPath, wgConfLatestRev)
		if err = cfg.SaveTo(latestConfig); err != nil {
			log.Fatal("Save latest configuration error", err)
		}
		if js.nodePubKeyInConfig {

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
	case darwin.String():
		activeDarwinConfig := filepath.Join(wgDarwinConfPath, wgConfActive)
		if err = cfg.SaveTo(activeDarwinConfig); err != nil {
			log.Fatal("Save latest configuration error", err)
		}
		if js.nodePubKeyInConfig {
			wgOut, err := runCommand("wg-quick", "down", wgIface)
			if err != nil {
				log.Printf("failed to start the wireguard interface: %v", err)
			}
			log.Printf("%v\n", wgOut)
			wgOut, err = runCommand("wg-quick", "up", activeDarwinConfig)
			if err != nil {
				log.Printf("failed to start the wireguard interface: %v", err)
			}
			log.Printf("%v\n", wgOut)
		} else {
			log.Printf("Tunnels not built since the node's public key was found in the configuration")
		}
	}
}