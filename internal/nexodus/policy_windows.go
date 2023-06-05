//go:build windows

package nexodus

// ProcessSecurityGroup for windows build purposes, policy currently unsupported on windows
func (ax *Nexodus) processSecurityGroupRules() error {
	return nil
}
