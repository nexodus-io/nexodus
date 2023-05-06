//go:build darwin

package nexodus

// ProcessSecurityGroup for darwin build purposes, policy currently unsupported on darwin
func (nx *Nexodus) processSecurityGroupRules() error {
	return nil
}

// nfNetworkRouterSetup for darwin build purposes, network router currently unsupported on darwin
func (nx *Nexodus) nfNetworkRouterSetup() error {
	return nil
}
