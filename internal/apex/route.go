package apex

import (
	"fmt"
	"net"

	"go.uber.org/zap"
)

// handlePeerRoute when a new configuration is deployed, delete/add the peer allowedIPs
func (ax *Apex) handlePeerRoute(wgPeerConfig wgPeerConfig) {
	switch ax.os {
	case Darwin.String():
		// Darwin maps to a utunX address which needs to be discovered
		netName, err := getInterfaceByIP(net.ParseIP(ax.wgLocalAddress))
		if err != nil {
			ax.logger.Debugf("failed to find the darwin interface with the address [ %s ] %v", ax.wgLocalAddress, err)
		}
		// If child prefix split the two prefixes (host /32 and child prefix
		for _, allowedIP := range wgPeerConfig.AllowedIPs {
			_, err := RunCommand("route", "-q", "-n", "delete", "-inet", allowedIP, "-interface", netName)
			if err != nil {
				ax.logger.Debugf("no route deleted: %v", err)
			}
			_, err = RunCommand("route", "-q", "-n", "add", "-inet", allowedIP, "-interface", netName)
			if err != nil {
				ax.logger.Debugf("child prefix route add failed: %v", err)
			}
		}

	case Linux.String():
		for _, allowedIP := range wgPeerConfig.AllowedIPs {
			routeExists, err := RouteExists(allowedIP)
			if err != nil {
				ax.logger.Info(err)
			}
			if !routeExists {
				if err := AddRoute(allowedIP, wgIface); err != nil {
					ax.logger.Errorf("route add failed: %v", err)
				}
			}
		}
	}
}

// addLinuxChildPrefixRoute check if the prefix exists and add it if not
func addLinuxChildPrefixRoute(prefix string) error {
	routeExists, err := RouteExists(prefix)
	if err != nil {
		return err
	}

	if !routeExists {
		if err := AddRoute(prefix, wgIface); err != nil {
			return fmt.Errorf("route add failed: %v", err)
		}
	}
	return nil
}

// addDarwinChildPrefixRoute todo: check if it exists
func addDarwinChildPrefixRoute(logger *zap.SugaredLogger, prefix string) error {
	_, err := RunCommand("route", "-q", "-n", "add", "-inet", prefix, "-interface", darwinIface)
	if err != nil {
		logger.Debugf("child-prefix route was added: %v", err)
	}
	return nil
}

func (ax *Apex) addChildPrefixRoute(ChildPrefix string) {
	routeExists, err := RouteExists(ChildPrefix)
	if err != nil {
		ax.logger.Warn(err)
	}

	if ax.os == Linux.String() && routeExists {
		ax.logger.Debugf("unable to add the child-prefix route [ %s ] as it already exists on this linux host", ChildPrefix)
		return
	}

	if ax.os == Linux.String() {
		if err := addLinuxChildPrefixRoute(ChildPrefix); err != nil {
			ax.logger.Infof("error adding the child prefix route: %v", err)
		}
	}

	// add osx child prefix
	if ax.os == Darwin.String() {
		if err := addDarwinChildPrefixRoute(ax.logger, ChildPrefix); err != nil {
			// TODO: setting to debug until the child prefix is looked up on Darwin
			ax.logger.Debugf("error adding the child prefix route: %v", err)
		}
	}
}
