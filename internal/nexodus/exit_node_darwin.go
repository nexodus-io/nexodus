//go:build darwin

package nexodus

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"net"
	"os/exec"
	"strings"
)

const (
	oobDNS           = 53
	oobHttps         = 443
	oobGoogleStun    = 19302
	wgFwMark         = 51820
	wgFwMarkStr      = "51820"
	oobFwMark        = "19302"
	oobFwdMarkHex    = "0x4B66"
	nfExitNodeTable  = "nexodus-exit-node"
	nfOobMangleTable = "nexodus-oob-mangle"
	nfOobSnatTable   = "nexodus-oob-snat"
)

// ExitNodeClientSetup setups up the routing tables, netfilter tables and out of band connections for the exit node client
func (nx *Nexodus) ExitNodeClientSetup() error {
	nx.exitNode.exitNodeClientEnabled = true

	// Lock deviceCache for read
	nx.deviceCacheLock.RLock()
	defer nx.deviceCacheLock.RUnlock()

	// Commented out until we are ready to support multiple exit nodes #1619
	// This code functions but adds a small delay in provisioning the exit node ~5s
	// that could likely be optimized or at least investigated.
	//exitNodeFound := false
	//for _, deviceEntry := range nx.deviceCache {
	//	for _, allowedIp := range deviceEntry.device.AdvertiseCidrs {
	//		if util.IsDefaultIPv4Route(allowedIp) {
	//			// assign the device advertising a default route as the exit node server/origin node
	//			localEndpoint := ""
	//			for _, endpoint := range deviceEntry.device.Endpoints {
	//				if endpoint.Source == "local" {
	//					localEndpoint = endpoint.Address
	//					break
	//				}
	//			}
	//			nx.exitNode.exitNodeOrigins[0] = wgPeerConfig{
	//				PublicKey:           deviceEntry.device.PublicKey,
	//				Endpoint:            localEndpoint,
	//				AllowedIPs:          deviceEntry.device.AllowedIps,
	//				PersistentKeepAlive: persistentKeepalive,
	//			}
	//			exitNodeFound = true
	//			break
	//		}
	//	}
	//	if exitNodeFound {
	//		break
	//	}
	//}

	if len(nx.exitNode.exitNodeOrigins) == 0 {
		return fmt.Errorf("no exit node found in this device's peerings")
	}

	gateway, err := getDefaultGatewayIPv4()
	if err != nil {
		nx.logger.Infof("Error getting default gateway: %v\n", err)
		return err
	}

	if err := AddRoute("0.0.0.0/1", "utun8"); err != nil {
		nx.logger.Infof("error adding exit node route 0.0.0.0/1: %v\n", err)
		return fmt.Errorf("error adding exit node route 0.0.0.0/1: %w", err)
	}

	if err := AddRoute("128.0.0.0/1", "utun8"); err != nil {
		nx.logger.Infof("error adding exit node route 128.0.0.0/1: %v\n", err)
		return fmt.Errorf("error adding exit node route 128.0.0.0/1: %w", err)
	}

	// TODO: do we bypass stun or let the peering checks find local peers?
	err = AddRouteForApiServer(nx.apiURL.String(), gateway)
	if err != nil {
		nx.logger.Infof("error adding exit node api-server bypass routes: %v", err)
		return fmt.Errorf("error adding exit node api-server bypass routes: %w", err)
	}

	if err := nx.handlePeerTunnel(nx.exitNode.exitNodeOrigins[0]); err != nil {
		nx.logger.Debug("error adding exit node peer tunnel", err)
		return err
	}

	if err := nx.handlePeerRouteOS(nx.exitNode.exitNodeOrigins[0]); err != nil {
		nx.logger.Debug("error adding exit node peer route", err)
		return err
	}

	// Loop through all other devices in deviceCache
	for _, deviceEntry := range nx.deviceCache {
		hasDefaultRoute := false
		for _, advertisedCIDR := range deviceEntry.device.AdvertiseCidrs {
			if advertisedCIDR == "0.0.0.0/0" {
				hasDefaultRoute = true
				break
			}
		}
		//
		//	// Skip if device has a default route
		if hasDefaultRoute {
			continue
		}

		// TODO: validate with multiple peers
		//address, err := getRelfexiveEndpointAddress(deviceEntry.device)
		//if err != nil {
		//	fmt.Println("Error:", err)
		//} else {
		//	fmt.Println("STUN Endpoint Address:", address)
		//}
		//
		////// Handle devices that don't have a default route
		//peer := wgPeerConfig{
		//	PublicKey:           deviceEntry.device.PublicKey,
		//	Endpoint:            deviceEntry.device.Endpoints[1].Address,
		//	AllowedIPs:          deviceEntry.device.AllowedIps,
		//	PersistentKeepAlive: persistentKeepalive,
		//}
		//
		//
		////if err := nx.handlePeerTunnel(peer); err != nil {
		////	nx.logger.Debug("Error handling peer tunnel for non-exit node devices: ", err)
		////	//return err
		////}
		//
		//
		////Add route to default gateway for this peer
		//if err := handlePeerEndpointRouteToGateway(peer.Endpoint, gateway); err != nil {
		//	nx.logger.Debug("Error adding route to gateway for peer: ", err)
		//	return err
		//}

	}

	nx.logger.Info("Exit node client configuration has been enabled")
	nx.logger.Debugf("Exit node client enabled and using the exit node server: %+v", nx.exitNode.exitNodeOrigins[0])

	return nil
}

func handlePeerEndpointRouteToGateway(peerEndpointIP, gatewayIP string) error {
	// Validate the peer endpoint IP
	peerIP := net.ParseIP(peerEndpointIP)
	if peerIP == nil {
		return fmt.Errorf("invalid peer endpoint IP: %s", peerEndpointIP)
	}

	// Validate the gateway IP
	gateway := net.ParseIP(gatewayIP)
	if gateway == nil {
		return fmt.Errorf("invalid gateway IP: %s", gatewayIP)
	}

	// Determine if IPv4 or IPv6
	var routeCommand string
	if strings.Contains(peerEndpointIP, ":") {
		// IPv6
		routeCommand = fmt.Sprintf("sudo route add -inet6 %s -gateway %s", peerEndpointIP, gatewayIP)
	} else {
		// IPv4
		routeCommand = fmt.Sprintf("sudo route -n add -inet %s %s", peerEndpointIP, gatewayIP)
	}

	// Execute the route command
	cmd := exec.Command("/bin/sh", "-c", routeCommand)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ailed to add route: %w", err)
	}

	return nil
}

// exitNodeOriginSetup build purposes only, origin nodes are not currently supported on Darwin
func (nx *Nexodus) exitNodeOriginSetup() error {
	return nil
}

func (nx *Nexodus) exitNodeClientTeardown() error {
	var err1, err2 error

	// TODO: this needs to be able to be set by nexctl but not for initial pre-deploy checks
	// nx.exitNode.exitNodeClientEnabled = false

	exitNodeRouteTables := []string{wgFwMarkStr, oobFwMark}
	for _, routeTable := range exitNodeRouteTables {
		if err1 = flushExitSrcRouteTableOOB(routeTable); err1 != nil {
			nx.logger.Debug(err1)
		}
	}

	exitNodeNFTables := []string{nfOobMangleTable, nfOobSnatTable}
	for _, nfTable := range exitNodeNFTables {
		if err2 = nx.policyTableDrop(nfTable); err2 != nil {
			nx.logger.Debug(err2)
		}
	}

	// If both functions return errors, concatenate their messages and return as a single error.
	if err1 != nil && err2 != nil {
		return fmt.Errorf("route table deletion error: %w, nftable deletion error: %w", err1, err2)
	}

	if err1 != nil {
		return err1
	}

	if err2 != nil {
		return err2
	}

	nx.logger.Info("Exit node client configuration has been disabled")

	return nil
}

// exitNodeOriginTeardown removes the exit node origin where traffic is originated when it exits the wireguard network
func (nx *Nexodus) exitNodeOriginTeardown() error {
	if err := nx.policyTableDrop(nfExitNodeTable); err != nil {
		return err
	}

	return nil
}

// getRelfexiveEndpointAddress takes a ModelsDevice and returns the address of the endpoint with source "stun".
func getRelfexiveEndpointAddress(device public.ModelsDevice) (string, error) {
	for _, endpoint := range device.Endpoints {
		if endpoint.Source == "stun" {
			return endpoint.Address, nil
		}
	}
	return "", fmt.Errorf("no STUN endpoint found")
}

// addTunBypassRoute adds a host route for destinations that need to bypass the wg exit node tunnel
func addTunBypassRoute(ip, gateway string) error {
	// Construct the route command with the provided IP address and gateway.
	cmd := exec.Command("sudo", "route", "-n", "add", ip, gateway)

	// Execute the route command.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add route: %s, output: %s", err, output)
	}

	fmt.Printf("Route to %s via %s added successfully.\n", ip, gateway)
	return nil
}

func lookupApiServerIP(url string) (string, error) {
	ips, err := net.LookupHost(url)
	if err != nil {
		return "", err
	}
	// Check if at least one IP address was returned
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found for %s", url)
	}
	// Return the first IP address.
	return ips[0], nil
}

// AddRouteForApiServer retrieves the API server's IP address and adds a route for it through the specified gateway.
func AddRouteForApiServer(apiURL, gateway string) error {
	apiServerIP, err := lookupApiServerIP(apiURL)
	if err != nil {
		return fmt.Errorf("failed to lookup API server IP: %s", err)
	}

	err = AddRoute(apiServerIP, gateway)
	if err != nil {
		return fmt.Errorf("failed to add route for API server IP %s: %s", apiServerIP, err)
	}

	return nil
}
