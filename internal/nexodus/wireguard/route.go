package wireguard

import "github.com/nexodus-io/nexodus/internal/models"

func (wg *WireGuard) AddChildPrefixRoute(childPrefix string) {

	routeExists, err := wg.routeExists(childPrefix)
	if err != nil {
		wg.Logger.Warn(err)
	}

	if routeExists {
		wg.Logger.Debugf("unable to add the child-prefix route [ %s ] as it already exists on this linux host", childPrefix)
		return
	}

	if err := addRoute(childPrefix, wg.TunnelIface); err != nil {
		wg.Logger.Infof("error adding the child prefix route: %v", err)
	}
}

func (wg *WireGuard) handlePeerRoute(wgPeerConfig WgPeerConfig) {
	if wg.UserspaceMode {
		wg.handlePeerRouteUS(wgPeerConfig)
	} else {
		wg.handlePeerRouteOS(wgPeerConfig)
	}
}

func (wg *WireGuard) handlePeerRouteDelete(dev string, wgPeerConfig models.Device) {
	if wg.UserspaceMode {
		wg.handlePeerRouteDeleteUS(dev, wgPeerConfig)
	} else {
		wg.handlePeerRouteDeleteOS(dev, wgPeerConfig)
	}
}

func (wg *WireGuard) routeExists(prefix string) (bool, error) {
	if wg.UserspaceMode {
		return routeExistsUS(prefix)
	} else {
		return routeExistsOS(prefix)
	}
}
