package wireguard

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/nexodus-io/nexodus/internal/models"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	wgBinary          = "wg"
	wgGoBinary        = "wireguard-go"
	wgWinBinary       = "wireguard.exe"
	WgLinuxConfPath   = "/etc/wireguard/"
	WgDarwinConfPath  = "/usr/local/etc/wireguard/"
	darwinIface       = "utun8"
	WgDefaultPort     = 51820
	wgIface           = "wg0"
	WgWindowsConfPath = "C:/nexd/"

	// wg keepalives are disabled and managed by the agent
	PersistentKeepalive    = "0"
	PersistentHubKeepalive = "0"
)

type WireGuard struct {
	WgConfig
	Peers  []WgPeerConfig `ini:"Peer,nonunique"`
	Relay  bool
	Logger *zap.SugaredLogger
	UserspaceWG
}

type WgConfig struct {
	WireguardPubKey         string
	WireguardPvtKey         string
	WireguardPubKeyInConfig bool
	TunnelIface             string
	ListenPort              int
	WgLocalAddress          string
	EndpointLocalAddress    string
}

type WgPeerConfig struct {
	PublicKey           string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepAlive string
}

// embedded in Nexodus struct
type UserspaceWG struct {
	UserspaceMode bool
	UserspaceTun  tun.Device
	UserspaceNet  *netstack.Net
	UserspaceDev  *device.Device
	// the last address configured on the userspace wireguard interface
	UserspaceLastAddress string
}

func NewWireGuard(wireguardPubKey string,
	wireguardPvtKey string,
	wgListenPort int,
	relay bool,
	logger *zap.SugaredLogger,
	userspaceWgMode bool,
) (*WireGuard, error) {

	var err error
	if err := binaryChecks(); err != nil {
		return nil, err
	}

	if err := checkOS(logger); err != nil {
		return nil, err
	}

	if wgListenPort == 0 {
		wgListenPort, err = GetWgListenPort()
		if err != nil {
			return nil, err
		}
	}

	wg := &WireGuard{
		WgConfig: WgConfig{WireguardPubKey: wireguardPubKey, WireguardPvtKey: wireguardPvtKey, ListenPort: wgListenPort},
		Peers:    []WgPeerConfig{},
		Relay:    relay,
		Logger:   logger,
		UserspaceWG: UserspaceWG{
			UserspaceMode: userspaceWgMode,
		},
	}

	wg.TunnelIface = defaultTunnelDev(wg.UserspaceMode)

	return wg, nil
}

// handlePeerTunnel build wg tunnels
func (wg *WireGuard) handlePeerTunnel(wgPeerConfig WgPeerConfig) {
	// validate the endpoint host:port pair parses.
	// temporary: currently if relay state has not converged the endpoint can be registered as (none)
	_, _, err := net.SplitHostPort(wgPeerConfig.Endpoint)
	if err != nil {
		wg.Logger.Debugf("failed parse the endpoint address for node [ %s ] (likely still converging) : %v\n", wgPeerConfig.PublicKey, err)
		return
	}

	if err := wg.addPeer(wgPeerConfig); err != nil {
		wg.Logger.Errorf("peer tunnel addition failed: %v\n", err)
	}
}

// addPeer add a wg peer
func (wg *WireGuard) addPeer(wgPeerConfig WgPeerConfig) error {
	if wg.UserspaceMode {
		return wg.addPeerUS(wgPeerConfig)
	}
	return wg.addPeerOS(wgPeerConfig)
}

// addPeerUs handles adding a new wireguard peer when using the userspace-only mode.
func (wg *WireGuard) addPeerUS(wgPeerConfig WgPeerConfig) error {
	// https://www.wireguard.com/xplatform/#configuration-protocol

	pubDecoded, err := base64.StdEncoding.DecodeString(wgPeerConfig.PublicKey)
	if err != nil {
		wg.Logger.Errorf("Failed to decode wireguard private key: %w", err)
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
	// See docs/development/design/nexodus-connectivity.md and internal/nexodus/keepalive.go
	// to see more about what Nexodus is doing instead of the built-in keepalive.
	// TODO Set this back to 0 once keepalive.go is updated to work with userspace mode.
	config += "persistent_keepalive_interval=5\n"
	wg.Logger.Debugf("Adding wireguard peer using: %s", config)
	err = wg.UserspaceDev.IpcSet(config)
	if err != nil {
		wg.Logger.Errorf("Failed to set wireguard config for new peer: %w", err)
		return err
	}
	fullConfig, err := wg.UserspaceDev.IpcGet()
	if err != nil {
		wg.Logger.Errorf("Failed to read back full wireguard config: %w", err)
		return nil
	}
	wg.Logger.Debugf("Updated config: %s", fullConfig)
	return nil
}

// addPeerOS configures a new wireguard peer when using an OS tun networking interface
func (wg *WireGuard) addPeerOS(wgPeerConfig WgPeerConfig) error {
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
		wg.Logger.Debugf("failed parse the endpoint address for node [ %s ] (likely still converging) : %v\n", wgPeerConfig.PublicKey, err)
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

	interval := time.Second * 0

	// relay nodes do not set explicit endpoints
	cfg := wgtypes.Config{}
	if wg.Relay {
		cfg = wgtypes.Config{
			ReplacePeers: false,
			Peers: []wgtypes.PeerConfig{
				{
					PublicKey:                   pubKey,
					Remove:                      false,
					AllowedIPs:                  allowedIP,
					PersistentKeepaliveInterval: &interval,
				},
			},
		}
	}
	// all other nodes set peer endpoints
	if !wg.Relay {
		cfg = wgtypes.Config{
			ReplacePeers: false,
			Peers: []wgtypes.PeerConfig{
				{
					PublicKey:                   pubKey,
					Remove:                      false,
					Endpoint:                    udpAddr,
					AllowedIPs:                  allowedIP,
					PersistentKeepaliveInterval: &interval,
				},
			},
		}
	}

	return wgClient.ConfigureDevice(wg.TunnelIface, cfg)
}

func (wg *WireGuard) HandlePeerDelete(device models.Device) error {
	wg.Logger.Debugf("Deleting peer with key: %s\n", device)
	if err := wg.deletePeer(device.PublicKey, wg.TunnelIface); err != nil {
		return fmt.Errorf("failed to delete peer: %w", err)
	}
	// delete the peer route(s)
	wg.handlePeerRouteDelete(wg.TunnelIface, device)
	return nil
}

func (wg *WireGuard) deletePeer(publicKey, dev string) error {
	if wg.UserspaceMode {
		return wg.deletePeerUS(publicKey)
	} else {
		return wg.deletePeerOS(publicKey, dev)
	}
}

// deletePeerUS deletes a wireguard peer when using a userspace device
func (wg *WireGuard) deletePeerUS(publicKey string) error {
	// https://www.wireguard.com/xplatform/#configuration-protocol

	pubDecoded, err := base64.StdEncoding.DecodeString(publicKey)
	if err != nil {
		wg.Logger.Errorf("Failed to decode wireguard private key: %w", err)
		return err
	}
	config := fmt.Sprintf("public_key=%s\nremove=true\n", hex.EncodeToString(pubDecoded))
	wg.Logger.Debugf("Removing wireguard peer using: %s", config)
	err = wg.UserspaceDev.IpcSet(config)
	if err != nil {
		wg.Logger.Errorf("Failed to remove wireguard peer (%s): %w", publicKey, err)
		return err
	}
	fullConfig, err := wg.UserspaceDev.IpcGet()
	if err != nil {
		wg.Logger.Errorf("Failed to read back full wireguard config: %w", err)
		return nil
	}
	wg.Logger.Debugf("Updated config: %s", fullConfig)
	return nil
}

// deletePeerOS deletes a wireguard peer when using an OS tun networking device
func (wg *WireGuard) deletePeerOS(publicKey, dev string) error {
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

	wg.Logger.Infof("Removed peer with key %s", key)
	return nil
}

func InPeerListing(peers []models.Device, p models.Device) bool {
	for _, peer := range peers {
		if peer.ID == p.ID {
			return true
		}
	}
	return false
}

func GetWgListenPort() (int, error) {
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
