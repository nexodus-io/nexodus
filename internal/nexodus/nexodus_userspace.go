package nexodus

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

func (ax *Nexodus) setupInterfaceUS() error {
	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{
			netip.MustParseAddr(ax.TunnelIP),
			netip.MustParseAddr(ax.TunnelIpV6),
		},
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
		ax.logger.Errorf("Failed to create userspace tunnel device: %w", err)
		return err
	}
	ax.userspaceTun = tun
	ax.userspaceNet = tnet
	logger := &device.Logger{
		Verbosef: device.DiscardLogf,
		Errorf:   ax.logger.Errorf,
	}
	if ax.logger.Level() == zap.DebugLevel {
		logger.Verbosef = ax.logger.Debugf
	}
	dev := device.NewDevice(ax.userspaceTun, conn.NewDefaultBind(), logger)
	pvtDecoded, err := base64.StdEncoding.DecodeString(ax.wireguardPvtKey)
	if err != nil {
		ax.logger.Errorf("Failed to decode wireguard private key: %w", err)
		return err
	}
	err = dev.IpcSet(fmt.Sprintf("private_key=%s", hex.EncodeToString(pvtDecoded)))
	if err != nil {
		ax.logger.Errorf("Failed to set private key: %w", err)
		return err
	}
	err = dev.IpcSet(fmt.Sprintf("listen_port=%d", ax.listenPort))
	if err != nil {
		ax.logger.Errorf("Failed to set listen port: %w", err)
		return err
	}
	err = dev.Up()
	if err != nil {
		ax.logger.Errorf("Failed to bring up userspace device: %w", err)
		return err
	}
	ax.userspaceDev = dev

	devName, err := tun.Name()
	if err != nil {
		ax.logger.Errorf("Failed to get userspace device name: %w", err)
		return err
	}
	ax.logger.Infof("Successfully created userspace device: %s", devName)
	ax.userspaceLastAddress = ax.TunnelIP

	return nil
}

func (ax *Nexodus) defaultTunnelDevUS() string {
	return defaultDeviceName
}
