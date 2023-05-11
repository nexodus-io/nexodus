//go:build windows

package stun

import (
	"net/netip"

	"go.uber.org/zap"
)

func Request(logger *zap.SugaredLogger, stunServer string, srcPort int) (netip.AddrPort, error) {
	return RequestWithReusePort(logger, stunServer, srcPort)
}
