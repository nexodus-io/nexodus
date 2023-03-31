package nexodus

import (
	"fmt"
)

// This is the hardcoded default name of the netstack wireguard device
const defaultDeviceName = "go"

func (ax *Nexodus) setupInterfaceUS() error {
	return fmt.Errorf("Not implemented")
}

func (ax *Nexodus) defaultTunnelDevUS() string {
	return defaultDeviceName
}
