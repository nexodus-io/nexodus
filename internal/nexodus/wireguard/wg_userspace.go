package wireguard

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/netip"

	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// This is the hardcoded default name of the netstack wireguard device
const defaultDeviceName = "go"

func (wg *WireGuard) setupInterfaceUS() error {
	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr(wg.WgLocalAddress)},
		// TODO - Is there something else that makes more sense as a DNS server?
		// So far I don't think DNS will ever be used. If Nexodus has its own
		// built-in DNS, that would make sense here.
		[]netip.Addr{netip.MustParseAddr("8.8.8.8")},
		// Assume a standard 1500 minus our tunneling overhead
		// TODO - make this configurable or dynamic. If there are
		// multiple layers of tunneling involved, it may need to be
		// even smaller. Consider running this inside a Kubernetes Pod,
		// where the cluster SDN provider has already reduced the MTU
		// to account for its own tunneling (Geneve, for example).
		1420)
	if err != nil {
		wg.Logger.Errorf("Failed to create userspace tunnel device: %w", err)
		return err
	}
	wg.UserspaceTun = tun
	wg.UserspaceNet = tnet
	logger := &device.Logger{
		Verbosef: device.DiscardLogf,
		Errorf:   wg.Logger.Errorf,
	}
	if wg.Logger.Level() == zap.DebugLevel {
		logger.Verbosef = wg.Logger.Debugf
	}
	dev := device.NewDevice(wg.UserspaceTun, conn.NewDefaultBind(), logger)
	pvtDecoded, err := base64.StdEncoding.DecodeString(wg.WireguardPvtKey)
	if err != nil {
		wg.Logger.Errorf("Failed to decode wireguard private key: %w", err)
		return err
	}
	err = dev.IpcSet(fmt.Sprintf("private_key=%s", hex.EncodeToString(pvtDecoded)))
	if err != nil {
		wg.Logger.Errorf("Failed to set private key: %w", err)
		return err
	}
	err = dev.IpcSet(fmt.Sprintf("listen_port=%d", wg.ListenPort))
	if err != nil {
		wg.Logger.Errorf("Failed to set listen port: %w", err)
		return err
	}
	err = dev.Up()
	if err != nil {
		wg.Logger.Errorf("Failed to bring up userspace device: %w", err)
		return err
	}
	wg.UserspaceDev = dev

	devName, err := tun.Name()
	if err != nil {
		wg.Logger.Errorf("Failed to get userspace device name: %w", err)
		return err
	}
	wg.Logger.Infof("Successfully created userspace device: %s", devName)
	wg.UserspaceLastAddress = wg.WgLocalAddress

	return nil
}

func defaultTunnelDevUS() string {
	return defaultDeviceName
}
