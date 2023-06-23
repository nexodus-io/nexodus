package nexodus

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/skupperproject/skupper/pkg/qdr"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
)

type SkupperConfigController struct {
	*Nexodus
	logger     *zap.SugaredLogger
	configFile string
}

func (cntlr *SkupperConfigController) Start(ctx context.Context, wg *sync.WaitGroup) error {
	util.GoWithWaitGroup(wg, func() {
		cntlr.reconcileLoop(ctx)
	})
	return nil
}

func (cntlr *SkupperConfigController) reconcileLoop(ctx context.Context) {
	reconcileTicker := time.NewTicker(time.Second * 20)
	defer reconcileTicker.Stop()

	lastErrorMsg := "none"
	errorMsg := ""
	for {

		select {
		case <-ctx.Done():
			return
		case <-reconcileTicker.C:
			err := cntlr.reconcile()
			if err != nil {
				errorMsg = err.Error()
				// avoid spamming the log with the same error over and over... only log when the error changes.
				if errorMsg != lastErrorMsg {
					cntlr.logger.Infow("skupper proxy configuration reconcile failed", "error", err)
				}
			} else {
				if lastErrorMsg != "" {
					cntlr.logger.Info("skupper proxy configuration reconcile succeeded")
				}
			}
			lastErrorMsg = errorMsg
		}
	}
}

func (cntlr *SkupperConfigController) reconcile() error {
	info, err := os.Stat(cntlr.configFile)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("skupper proxy configuration file is a director")
	}

	configBytes, err := os.ReadFile(cntlr.configFile)
	if err != nil {
		return err
	}

	config, err := qdr.UnmarshalRouterConfig(string(configBytes))
	if err != nil {
		return err
	}

	desiredRules, err := cntlr.toProxyRulesFromSkupperConfig(config)
	if err != nil {
		return fmt.Errorf("could not convert skupper config to proxy rules: %w", err)
	}

	currentRules, err := cntlr.listProxyRules()
	if err != nil {
		return fmt.Errorf("could not get nexodus proxy rules: %w", err)
	}

	diffSet := map[ProxyRule]struct{}{}
	for _, rule := range currentRules {
		diffSet[rule] = struct{}{}
	}

	for _, rule := range desiredRules {
		if _, found := diffSet[rule]; found {
			// take it out, anything remaining in the diffSet,
			// will be rules that need to be removed.
			delete(diffSet, rule)
		} else {
			// add the rule..
			proxy, err := cntlr.UserspaceProxyRemove(rule)
			if err != nil {
				return err
			}
			proxy.Start(cntlr.nexCtx, cntlr.nexWg, cntlr.userspaceNet)
		}
	}

	for rule := range diffSet {
		// remove the rule
		proxy, err := cntlr.UserspaceProxyAdd(rule)
		if err != nil {
			return err
		}
		proxy.Stop()
	}

	return nil
}

func (cntlr *SkupperConfigController) toProxyRulesFromSkupperConfig(config qdr.RouterConfig) ([]ProxyRule, error) {
	var result []ProxyRule

	for _, endpoint := range config.Bridges.TcpConnectors {

		port, err := parsePort(endpoint.Port)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint (%s) port: %w", endpoint.Name, err)
		}

		listenPort, err := cntlr.getIngressPortForAddress(endpoint.Address)
		if err != nil {
			return nil, err
		}

		result = append(result, ProxyRule{
			ProxyKey: ProxyKey{
				ruleType:   ProxyTypeIngress,
				protocol:   proxyProtocolTCP,
				listenPort: listenPort,
			},
			dest: HostPort{
				host: endpoint.Host,
				port: port,
			},
			stored: false,
		})

	}

	for _, endpoint := range config.Bridges.TcpListeners {

		listenPort, err := parsePort(endpoint.Port)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint (%s) port: %w", endpoint.Name, err)
		}

		destinations, err := cntlr.getServiceDestinations(endpoint.Address)
		if err != nil {
			return nil, err
		}

		for _, dest := range destinations {
			result = append(result, ProxyRule{
				ProxyKey: ProxyKey{
					ruleType:   ProxyTypeEgress,
					protocol:   proxyProtocolTCP,
					listenPort: listenPort,
				},
				dest:   dest,
				stored: false,
			})
		}
	}

	return result, nil
}

func (cntlr *SkupperConfigController) getIngressPortForAddress(address string) (int, error) {

	// find if we have allocated a port of the address before,
	// if not, then allocate one and update our address/port mappings on the apiserver

	return 0, nil
}

func (cntlr *SkupperConfigController) getServiceDestinations(address string) ([]HostPort, error) {
	var result []HostPort

	// get all the address/port mappings for all node peers and find which nodes
	// are hosing the addresses

	return result, nil
}

func (cntlr *SkupperConfigController) listProxyRules() ([]ProxyRule, error) {
	var result []ProxyRule
	cntlr.proxyLock.RLock()
	defer cntlr.proxyLock.RUnlock()
	for _, proxy := range cntlr.proxies {
		proxy.mu.RLock()
		for _, rule := range proxy.rules {
			result = append(result, rule)
		}
		proxy.mu.RUnlock()
	}
	return result, nil
}
