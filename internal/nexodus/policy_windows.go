//go:build windows

package nexodus

import (
	"go.uber.org/zap"
)

// processSecurityGroupRules for windows build purposes, policy currently unsupported on windows
func (nx *Nexodus) processSecurityGroupRules() error {
	return nil
}

// networkRouterSetup for windows build purposes, network router currently unsupported on windows
func (nx *Nexodus) networkRouterSetup() error {
	return nil
}

// policyCmd for windows build purposes
func policyCmd(logger *zap.SugaredLogger, cmd []string) (string, error) {
	return "", nil
}

// policyTableDrop for windows build purposes
func (nx *Nexodus) policyTableDrop(table string) error {
	return nil
}
