package nexodus

import (
	"fmt"
	"github.com/bytedance/gopkg/util/logger"
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
	for _, proxy := range ac.ax.proxies {
		proxy.mu.RLock()
		for _, rule := range proxy.rules {
			*result += fmt.Sprintf("%s\n", rule.AsFlag())
		}
		proxy.mu.RUnlock()
	}
	return nil
}

func (ac *NexdCtl) proxyAdd(proxyType ProxyType, rule string, result *string) error {

	proxyRule, err := ParseProxyRule(rule, proxyType)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to parse %s proxy rule (%s): %v", proxyType, rule, err))
	}
	proxyRule.stored = true

	proxy, err := ac.ax.UserspaceProxyAdd(proxyRule)
	if err != nil {
		return err
	}
	proxy.Start(ac.ax.nexCtx, ac.ax.nexWg, ac.ax.userspaceNet)

	err = ac.ax.StoreProxyRules()
	if err != nil {
		return err
	}
	*result = fmt.Sprintf("Added %s proxy rule: %s\n", proxyType, rule)
	return nil
}

func (ac *NexdCtl) ProxyAddIngress(rule string, result *string) error {
	return ac.proxyAdd(ProxyTypeIngress, rule, result)
}

func (ac *NexdCtl) ProxyAddEgress(rule string, result *string) error {
	return ac.proxyAdd(ProxyTypeEgress, rule, result)
}

func (ac *NexdCtl) proxyRemove(proxyType ProxyType, rule string, result *string) error {
	proxyRule, err := ParseProxyRule(rule, proxyType)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to parse %s proxy rule (%s): %v", proxyType, rule, err))
	}
	proxyRule.stored = true

	_, err = ac.ax.UserspaceProxyRemove(proxyRule)
	if err != nil {
		return err
	}
	err = ac.ax.StoreProxyRules()
	if err != nil {
		return err
	}

	*result = fmt.Sprintf("Removed ingress proxy rule: %s\n", rule)
	return nil
}
func (ac *NexdCtl) ProxyRemoveIngress(rule string, result *string) error {
	return ac.proxyRemove(ProxyTypeIngress, rule, result)
}

func (ac *NexdCtl) ProxyRemoveEgress(rule string, result *string) error {
	return ac.proxyRemove(ProxyTypeEgress, rule, result)
}
