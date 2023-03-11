package nexodus

func (ax *Nexodus) addChildPrefixRoute(childPrefix string) {

	routeExists, err := RouteExists(childPrefix)
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
