//go:build windows

package nexodus

// ProcessSecurityGroup for windows build purposes, policy currently unsupported on windows
func (nx *Nexodus) processSecurityGroupRules() error {
	return nil
}

// nfNetworkRouterSetup for windows build purposes, network router currently unsupported on windows
func (nx *Nexodus) nfNetworkRouterSetup() error {
	return nil
}
