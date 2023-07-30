//go:build darwin

package nexodus

import "go.uber.org/zap"

// ProcessSecurityGroup for darwin build purposes, policy currently unsupported on darwin
func (nx *Nexodus) processSecurityGroupRules() error {
	return nil
}

// nfNetworkRouterSetup for darwin build purposes, network router currently unsupported on darwin
func (nx *Nexodus) nfNetworkRouterSetup() error {
	return nil
}

// runNftCmd for Darwin build purposes
func runNftCmd(logger *zap.SugaredLogger, cmd []string) (string, error) {
	return "", nil
}

// nfTableDrop for Darwin build purposes
func (nx *Nexodus) nfTableDrop(table string) error {
	return nil
}
