//go:build windows

package nexodus

// ExitNodeClientSetup setups up the routing tables, netfilter tables and out of band connections for the exit node client
func (nx *Nexodus) ExitNodeClientSetup() error {
	return nil
}

// exitNodeOriginSetup sets up the exit node origin where traffic is originated when it exits the wireguard network
func (nx *Nexodus) exitNodeOriginSetup() error {
	return nil
}

// exitNodeOriginTeardown removes the exit node origin where traffic is originated when it exits the wireguard network
func (nx *Nexodus) exitNodeOriginTeardown() error {
	return nil
}
