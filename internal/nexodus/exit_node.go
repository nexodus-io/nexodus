package nexodus

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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

	exitNodeFound := false
	for _, deviceEntry := range nx.deviceCache {
		for _, allowedIp := range deviceEntry.device.AllowedIps {
			if util.IsDefaultIPv4Route(allowedIp) {
				// assign the device advertising a default route as the exit node server/origin node
				localEndpoint := ""
				for _, endpoint := range deviceEntry.device.Endpoints {
					if endpoint.GetSource() == "local" {
						localEndpoint = endpoint.GetAddress()
						break
					}
				}
				nx.exitNode.exitNodeOrigins[0] = wgPeerConfig{
					PublicKey:           deviceEntry.device.GetPublicKey(),
					Endpoint:            localEndpoint,
					AllowedIPs:          deviceEntry.device.AllowedIps,
					PersistentKeepAlive: persistentKeepalive,
				}
				exitNodeFound = true
				break
			}
		}
		if exitNodeFound {
			break
		}
	}

	if len(nx.exitNode.exitNodeOrigins) == 0 {
		return fmt.Errorf("no exit node found in this device's peerings")
	}

	// teardown any residual nf tables or routing tables from previous runs
	if err := nx.exitNodeClientTeardown(); err != nil {
		nx.logger.Debug(err)
	}

	// Add a fwMark to the wg interface
	c, err := wgctrl.New()
	if err != nil {
		nx.logger.Errorf("could not connect to wireguard: %v\n", err)
		return fmt.Errorf("%w", interfaceErr)
	}
	defer util.IgnoreError(c.Close)

	fwMark := wgFwMark
	err = c.ConfigureDevice(nx.tunnelIface, wgtypes.Config{
		ReplacePeers: false,
		FirewallMark: &fwMark,
	})

	if err != nil {
		return fmt.Errorf("error adding exit node client fwdMark: %w", err)
	}

	if err := nx.handlePeerTunnel(nx.exitNode.exitNodeOrigins[0]); err != nil {
		nx.logger.Debug(err)
		return err
	}

	devName, err := getInterfaceFromIPv4(nx.endpointLocalAddress)
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
	}

	if err := nfAddExitSrcMangleTable(nx.logger); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcMangleOutputChain(nx.logger); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcDestPortMangleRule(nx.logger, "udp", oobDNS); err != nil {
		nx.logger.Debug(err)
		return err
	}

	ips, err := ResolveURLToIP(nx.apiURL.String())
	if err != nil {
		nx.logger.Debug(err)
		return err
	}

	for _, ip := range ips {
		if err := nfAddExitSrcApiServerOOBMangleRule(nx.logger, "tcp", ip.String(), oobHttps); err != nil {
			fmt.Printf("Error adding rule for IP %s: %v\n", ip, err)
		}
	}

	if err := nfAddExitSrcDestPortMangleRule(nx.logger, "udp", oobGoogleStun); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatTable(nx.logger); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatOutputChain(nx.logger); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatOutputChain(nx.logger); err != nil {
		nx.logger.Debug(err)
		return err
	}

	if err := nfAddExitSrcSnatRule(nx.logger, devName); err != nil {
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

// exitNodeOriginSetup sets up the exit node origin where traffic is originated when it exits the wireguard network
func (nx *Nexodus) exitNodeOriginSetup() error {
	// clean up any existing exit-node tables from previous executions
	if err := nx.exitNodeOriginTeardown(); err != nil {
		return err
	}

	devName, err := getInterfaceFromIPv4(nx.endpointLocalAddress)
	if err != nil {
		nx.logger.Debugf("failed to discover the interface with the address [ %s ] %v", nx.endpointLocalAddress, err)
	}

	if err := addExitDestinationTable(nx.logger); err != nil {
		return err
	}

	if err := addExitOriginPreroutingChain(nx.logger); err != nil {
		return err
	}

	if err := addExitOriginPostroutingChain(nx.logger); err != nil {
		return err
	}

	if err := addExitOriginForwardChain(nx.logger); err != nil {
		return err
	}

	if err := addExitOriginPostroutingRule(nx.logger, devName); err != nil {
		return err
	}

	if err := addExitOriginForwardRule(nx.logger); err != nil {
		return err
	}

	nx.logger.Debug("Exit node server enabled on this node")

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
