package aircrew

import (
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/ini.v1"
)

const (
	aircrewConfig       = "endpoints.toml"
	persistentKeepalive = "25"
)

type AircrewState struct {
	NodePubKey         string
	NodePvtKey         string
	NodePubKeyInConfig bool
	AircrewConfigFile  string
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
}

type ConfigToml struct {
	Peers map[string]PeerToml `mapstructure:"Peers"`
}

// TODO: add support for AllowedIPs as a []list
type PeerToml struct {
	PublicKey   string `mapstructure:"PublicKey"`
	PrivateKey  string `mapstructure:"PrivateKey"`
	WireguardIP string `mapstructure:"AllowedIPs"`
	EndpointIP  string `mapstructure:"EndpointIP"`
}

// parseAircrewConfig extracts the aircrew toml config and
// builds the wireguard configuration data structs
func (as *AircrewState) ParseAircrewConfig() {
	// parse toml config
	viper.SetConfigType("toml")
	viper.SetConfigFile(as.AircrewConfigFile)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("Unable to read config file", err)
	}
	var conf ConfigToml
	err := viper.Unmarshal(&conf)
	if err != nil {
		log.Fatal(err)
	}

	var peers []wgPeerConfig
	var localInterface wgLocalConfig

	for _, value := range conf.Peers {
		if value.PublicKey == as.NodePubKey {
			as.NodePubKeyInConfig = true
		}
	}
	if !as.NodePubKeyInConfig {
		log.Printf("Public Key for this node was not found in %s", aircrewConfig)
	}
	for nodeName, value := range conf.Peers {
		// Parse the [Peers] section
		if value.PublicKey != as.NodePubKey {
			peer := wgPeerConfig{
				value.PublicKey,
				value.EndpointIP,
				value.WireguardIP,
				persistentKeepalive,
			}
			peers = append(peers, peer)
			log.Printf("Peer Node Configuration [%v] Peer AllowedIPs [%s] Peer Endpoint IP [%s] Peer Public Key [%s]\n",
				nodeName,
				value.WireguardIP,
				value.EndpointIP,
				value.PublicKey)
		}
		// Parse the [Interface] section of the wg config
		if value.PublicKey == as.NodePubKey {
			localInterface = wgLocalConfig{
				value.PrivateKey,
				value.WireguardIP,
				WgListenPort,
				false,
			}
			log.Infof("Local Node Configuration Name [%v] Wireguard Address [%v] Local Endpoint IP [%v] Local Private Key [%v]\n",
				nodeName,
				value.WireguardIP,
				value.EndpointIP,
				value.PrivateKey)
		}
	}
	as.WgConf.Interface = localInterface
	as.WgConf.Peer = peers
}

func (as *AircrewState) DeployWireguardConfig() {
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
		// wg does not create the OSX config directory by default
		if err = CreateDirectory(WgLinuxConfPath); err != nil {
			log.Fatalf("Unable to create the wireguard config directory [%s]: %v", WgDarwinConfPath, err)
		}

		latestConfig := filepath.Join(WgLinuxConfPath, wgConfLatestRev)
		if err = cfg.SaveTo(latestConfig); err != nil {
			log.Fatal("Save latest configuration error", err)
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
	case Linux.String():
		activeDarwinConfig := filepath.Join(WgDarwinConfPath, wgConfActive)
		if err = cfg.SaveTo(activeDarwinConfig); err != nil {
			log.Fatal("Save latest configuration error", err)
		}

		if as.NodePubKeyInConfig {
			// this will throw an error that can be ignored if an existing interface doesn't exist
			wgOut, err := RunCommand("wg-quick", "down", wgIface)
			if err != nil {
				log.Debugf("failed to start the wireguard interface: %v", err)
			}
			log.Debugf("%v\n", wgOut)
			wgOut, err = RunCommand("wg-quick", "up", activeDarwinConfig)
			if err != nil {
				log.Printf("failed to start the wireguard interface: %v", err)
			}
			log.Debugf("%v\n", wgOut)
		} else {
			log.Printf("Tunnels not built since the node's public key was found in the configuration")
		}
	}
}
