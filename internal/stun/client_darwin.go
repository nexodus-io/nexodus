//go:build darwin

package stun

import (
	"go.uber.org/zap"
	"net/netip"
)

func Request(logger *zap.SugaredLogger, stunServer string, srcPort int) (netip.AddrPort, error) {
	return RequestWithReusePort(logger, stunServer, srcPort)
}
