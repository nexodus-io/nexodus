package nexodus

import "github.com/nexodus-io/nexodus/internal/models"

func (ax *Nexodus) addChildPrefixRoute(childPrefix string) {

	routeExists, err := ax.RouteExists(childPrefix)
	if err != nil {
		ax.logger.Warn(err)
	}

	if routeExists {
		ax.logger.Debugf("unable to add the child-prefix route [ %s ] as it already exists on this linux host", childPrefix)
		return
	}

	if err := AddRoute(childPrefix, ax.tunnelIface); err != nil {
		ax.logger.Infof("error adding the child prefix route: %v", err)
	}
}

func (ax *Nexodus) handlePeerRoute(wgPeerConfig wgPeerConfig) {
	if ax.userspaceMode {
		ax.handlePeerRouteUS(wgPeerConfig)
	} else {
		ax.handlePeerRouteOS(wgPeerConfig)
	}
}

func (ax *Nexodus) handlePeerRouteDelete(dev string, wgPeerConfig models.Device) {
	if ax.userspaceMode {
		ax.handlePeerRouteDeleteUS(dev, wgPeerConfig)
	} else {
		ax.handlePeerRouteDeleteOS(dev, wgPeerConfig)
	}
}

func (ax *Nexodus) RouteExists(prefix string) (bool, error) {
	if ax.userspaceMode {
		return RouteExistsUS(prefix)
	} else {
		return RouteExistsOS(prefix)
	}
}
