package nexodus

import (
	"github.com/nexodus-io/nexodus/internal/api/public"
)

func (nx *Nexodus) handlePeerRoute(wgPeerConfig wgPeerConfig) error {
	if nx.userspaceMode {
		return nx.handlePeerRouteUS(wgPeerConfig)
	} else {
		return nx.handlePeerRouteOS(wgPeerConfig)
	}
}

func (nx *Nexodus) handlePeerRouteDelete(dev string, wgPeerConfig public.ModelsDevice) {
	if nx.userspaceMode {
		nx.handlePeerRouteDeleteUS(dev, wgPeerConfig)
	} else {
		nx.handlePeerRouteDeleteOS(dev, wgPeerConfig)
	}
}

func (nx *Nexodus) RouteExists(prefix string) (bool, error) {
	if nx.userspaceMode {
		return RouteExistsUS(prefix)
	} else {
		return RouteExistsOS(prefix)
	}
}
