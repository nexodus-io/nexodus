package nexodus

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
)

const (
	oobDNS        = 53
	oobHttps      = 443
	oobGoogleStun = 19302
)

// ExitNodeClientSetup setups up the routing tables, netfilter tables and out of band connections for the exit node client
func (nx *Nexodus) ExitNodeClientSetup() error {
	nx.exitNode.exitNodeClientEnabled = true

	// Lock deviceCache for read
	nx.deviceCacheLock.RLock()
	defer nx.deviceCacheLock.RUnlock()

	for _, deviceEntry := range nx.deviceCache {
		for _, allowedIp := range deviceEntry.device.AllowedIps {
			if util.IsDefaultIPv4Route(allowedIp) {
				// assign the device advertising a default route as the exit node server/origin node
				nx.exitNode.exitNodeOrigins[0] = wgPeerConfig{
					PublicKey:           deviceEntry.device.PublicKey,
					Endpoint:            deviceEntry.device.EndpointLocalAddressIp4,
					AllowedIPs:          deviceEntry.device.AllowedIps,
					PersistentKeepAlive: "25",
				}
			}
		}
	}

	if len(nx.exitNode.exitNodeOrigins) == 0 {
		return fmt.Errorf("no exit node found in this device's peerings")
	}

	nx.handlePeerTunnel(nx.exitNode.exitNodeOrigins[0])

	devName, err := getInterfaceFromIP(nx.endpointLocalAddress)
	if err != nil {
		nx.logger.Debugf("failed to discover the interface with the address [ %s ] %v", nx.endpointLocalAddress, err)
	}

	if err := enableExitSrcValidMarkV4(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := addExitSrcRuleToRPDB(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := addExitSrcRuleIgnorePrefixLength(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := addExitSrcDefaultRouteTable(); err != nil {
		nx.logger.Debug(err)
		nx.logger.Debugf("default route already exists in table %s", oobFwMark)
		//return err
	}

	if err := nfAddExitSrcMangleTable(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcMangleOutputChain(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcDestPortMangleRule("udp", oobDNS); err != nil {
		nx.logger.Debug(err)
		return err
	}

	ips, err := ResolveURLToIP(nx.apiURL.String())
	if err != nil {
		nx.logger.Debug(err)
		return err
	}

	for _, ip := range ips {
		err := nfAddExitSrcApiServerOOBMangleRule("tcp", ip.String(), oobHttps)
		if err != nil {
			fmt.Printf("Error adding rule for IP %s: %v\n", ip, err)
		}
	}

	if err := nfAddExitSrcDestPortMangleRule("udp", oobGoogleStun); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatTable(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatOutputChain(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatOutputChain(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatRule(devName); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := addExitSrcDefaultRouteTableOOB(devName); err != nil {
		nx.logger.Debugf("default route already exists in table %s", oobFwMark)
	}

	if err := addExitSrcRuleFwMarkOOB(); err != nil {
		nx.logger.Debug(err)
		return err
	}

	nx.logger.Info("Exit node client configuration has been enabled")
	nx.logger.Debugf("Exit node client enabled and using the exit node server: %+v", nx.exitNode.exitNodeOrigins[0])

	return nil
}

func (nx *Nexodus) exitNodeClientTeardown() error {

	return nil
}

// exitNodeOriginSetup sets up the exit node origin where traffic is originated when it exits the wireguard network
func (nx *Nexodus) exitNodeOriginSetup() error {
	devName, err := getInterfaceFromIP(nx.endpointLocalAddress)
	if err != nil {
		nx.logger.Debugf("failed to discover the interface with the address [ %s ] %v", nx.endpointLocalAddress, err)
	}

	if err := addExitDestinationTable(); err != nil {
		return err
	}

	if err := addExitOriginPreroutingChain(); err != nil {
		return err
	}

	if err := addExitOriginPostroutingChain(); err != nil {
		return err
	}

	if err := addExitOriginForwardChain(); err != nil {
		return err
	}

	if err := addExitOriginPostroutingRule(devName); err != nil {
		return err
	}

	if err := addExitOriginForwardRule(); err != nil {
		return err
	}

	nx.logger.Debug("Exit node server enabled on this node")

	return nil
}

func (nx *Nexodus) ExitNodeOriginTeardown() error {
	var err1, err2 error
	nx.exitNode.exitNodeClientEnabled = false

	exitNodeRouteTables := []string{wgFwMark, oobFwMark}
	for _, routeTable := range exitNodeRouteTables {
		err1 = flushExitSrcRouteTableOOB(routeTable)
		if err1 != nil {
			nx.logger.Debug(err1)

		}
	}

	exitNodeNFTables := []string{nfOobMangleTable, nfOobSnatTable}
	for _, nfTable := range exitNodeNFTables {
		err2 = deleteExitSrcTable(nfTable)
		if err2 != nil {
			nx.logger.Debug(err2)
		}
	}

	// If both functions return errors, concatenate their messages and return as a single error.
	if err1 != nil && err2 != nil {
		return fmt.Errorf("route table deletion error: %v, nftable deletion error: %v", err1, err2)
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
