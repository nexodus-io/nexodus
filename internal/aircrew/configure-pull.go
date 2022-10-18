package aircrew

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/redhat-et/jaywalking/internal/messages"
	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

const (
	persistentKeepalive = "25"
)

type AircrewState struct {
	NodePubKey         string
	NodePvtKey         string
	NodePubKeyInConfig bool
	AircrewConfigFile  string
	HubRouter          bool
	Daemon             bool
	NodeOS             string
	Zone               string
	RequestedIP        string
	ChildPrefix        string
	AgentChannel       string
	UserEndpointIP     string
	WgConf             wgConfig
}

type wgConfig struct {
	Interface wgLocalConfig
	Peer      []wgPeerConfig `ini:",nonunique"`
}

type wgPeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          string
	PersistentKeepAlive string
	// AllowedIPs []string `delim:","` TODO: support an AllowedIPs slice here
}

type wgLocalConfig struct {
	PrivateKey string
	Address    string
	ListenPort int
	SaveConfig bool
	PostUp     string
	PostDown   string
}

// parseAircrewControlTowerConfig this is hacky but assumes there is no local config
// or if there is will overwrite it from the publisher peer listing
func (as *AircrewState) ParseAircrewControlTowerConfig(listenPort int, peerListing []messages.Peer) {

	var peers []wgPeerConfig
	var localInterface wgLocalConfig
	var hubRouterExists bool
	for _, value := range peerListing {
		if value.PublicKey == as.NodePubKey {
			as.NodePubKeyInConfig = true
		}
		if value.HubRouter {
			hubRouterExists = true
		}
	}
	if !as.NodePubKeyInConfig {
		log.Printf("Public Key for this node %s was not found in the control tower update\n", as.NodePubKey)
	}
	// determine if the peer listing for this node is a hub zone or hub-router
	for _, value := range peerListing {
		if value.PublicKey == as.NodePubKey && value.HubRouter {
			log.Debug("This node is a hub-router")
			if as.NodeOS == Darwin.String() {
				log.Fatalf("OSX nodes are not supported as bouncer hubs")
			} else {
				as.deployControlTowerHubWireguardConfig(listenPort, peerListing)
				return
			}
		}
		if value.HubZone {
			log.Debug("This zone is a hub-zone")
			if !hubRouterExists {
				log.Error("Cannot deploy to a hub-zone if no hub router has joined the zone yet. See `--hub-router`")
				os.Exit(1)
			}
			as.deployControlTowerHubWireguardConfig(listenPort, peerListing)
			return

		}
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
				"",
				"",
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
