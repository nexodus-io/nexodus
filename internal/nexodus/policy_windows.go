//go:build windows

package nexodus

import (
	"fmt"
	"go.uber.org/zap"
)

// ProcessSecurityGroup for windows build purposes, policy currently unsupported on windows
func (nx *Nexodus) processSecurityGroupRules() error {
	return nil
}

// nfNetworkRouterSetup for windows build purposes, network router currently unsupported on windows
func (nx *Nexodus) nfNetworkRouterSetup() error {
	return nil
}

// runNftCmd for windows build purposes
func runNftCmd(logger *zap.SugaredLogger, cmd []string) (string, error) {
	return "", nil
}

// nfTableDrop for windows build purposes
func (nx *Nexodus) nfTableDrop(table string) error {
	return fmt.Errorf("nft table drop method currently unsupported for windows")
}
