//go:build darwin

package nexodus

// ProcessSecurityGroup for darwin build purposes, policy currently unsupported on darwin
func (ax *Nexodus) processSecurityGroupRules() error {
	return nil
}
