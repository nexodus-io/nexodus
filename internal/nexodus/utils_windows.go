//go:build windows

package nexodus

import (
	"net"
	"net/url"

	"go.uber.org/zap"
)

// discoverLinuxAddress only used for windows build purposes
func discoverLinuxAddress(logger *zap.SugaredLogger, family int) (net.IP, error) {
	return nil, nil
}

func findLocalIP(logger *zap.SugaredLogger, controllerURL *url.URL) (string, error) {
	return discoverGenericIPv4(logger, controllerURL.Host, "443")
}
