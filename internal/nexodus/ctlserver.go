package nexodus

import (
	"fmt"
)

type NexdCtl struct {
	ax *Nexodus
}

func (ac *NexdCtl) Status(_ string, result *string) error {
	var statusStr string
	switch ac.ax.status {
	case NexdStatusStarting:
		statusStr = "Starting"
	case NexdStatusAuth:
		statusStr = "WaitingForAuth"
	case NexdStatusRunning:
		statusStr = "Running"
	default:
		statusStr = "Unknown"
	}
	res := fmt.Sprintf("Status: %s\n", statusStr)
	if len(ac.ax.statusMsg) > 0 {
		res += ac.ax.statusMsg
	}
	*result = res
	return nil
}

func (ac *NexdCtl) Version(_ string, result *string) error {
	*result = ac.ax.version
	return nil
}

func (ac *NexdCtl) GetTunnelIPv4(_ string, result *string) error {
	*result = ac.ax.TunnelIP
	return nil
}

func (ac *NexdCtl) GetTunnelIPv6(_ string, result *string) error {
	*result = ac.ax.TunnelIpV6
	return nil
}

func (ac *NexdCtl) ProxyList(_ string, result *string) error {
	*result = ""
	ac.ax.proxyLock.RLock()
	defer ac.ax.proxyLock.RUnlock()
	for _, proxy := range ac.ax.ingressProxies {
		*result += fmt.Sprintf("%s\n", proxy)
	}
	for _, proxy := range ac.ax.egressProxies {
		*result += fmt.Sprintf("%s\n", proxy)
	}
	return nil
}

func (ac *NexdCtl) ProxyAddIngress(rule string, result *string) error {
	proxy, err := ac.ax.UserspaceProxyAdd(ac.ax.nexCtx, ac.ax.nexWg, rule, ProxyTypeIngress, true)
	if err != nil {
		return err
	}
	proxy.stored = true
	err = ac.ax.StoreProxyRules()
	if err != nil {
		return err
	}
	*result = fmt.Sprintf("Added ingress proxy rule: %s\n", rule)
	return nil
}

func (ac *NexdCtl) ProxyAddEgress(rule string, result *string) error {
	proxy, err := ac.ax.UserspaceProxyAdd(ac.ax.nexCtx, ac.ax.nexWg, rule, ProxyTypeEgress, true)
	if err != nil {
		return err
	}
	proxy.stored = true
	err = ac.ax.StoreProxyRules()
	if err != nil {
		return err
	}
	*result = fmt.Sprintf("Added egress proxy rule: %s\n", rule)
	return nil
}

func (ac *NexdCtl) ProxyRemoveIngress(rule string, result *string) error {
	proxy, err := ac.ax.UserspaceProxyRemove(rule, ProxyTypeIngress)
	if err != nil {
		return err
	}
	if proxy.stored {
		err = ac.ax.StoreProxyRules()
		if err != nil {
			return err
		}
	}
	*result = fmt.Sprintf("Removed ingress proxy rule: %s\n", rule)
	return nil
}

func (ac *NexdCtl) ProxyRemoveEgress(rule string, result *string) error {
	proxy, err := ac.ax.UserspaceProxyRemove(rule, ProxyTypeEgress)
	if err != nil {
		return err
	}
	if proxy.stored {
		err = ac.ax.StoreProxyRules()
		if err != nil {
			return err
		}
	}
	*result = fmt.Sprintf("Removed egress proxy rule: %s\n", rule)
	return nil
}
