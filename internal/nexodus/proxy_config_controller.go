package nexodus

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/skupperproject/skupper/pkg/qdr"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type AddressKey struct {
	protocol string
	address  string
}

const lowPortNumber = 1024
const highPortNumber = 1024 * 32

type SkupperConfigController struct {
	*Nexodus
	logger               *zap.SugaredLogger
	configFile           string
	metadataInformer     *public.ApiListOrganizationMetadataInformer
	lastErrorMsg         string
	ingressPortToAddress map[HostPort]string
	ingressAddressToPort map[AddressKey]int
	nextPort             int
}

func (cntlr *SkupperConfigController) Start(ctx context.Context, wg *sync.WaitGroup) error {
	util.GoWithWaitGroup(wg, func() {
		cntlr.reconcileLoop(ctx)
	})
	return nil
}

func (cntlr *SkupperConfigController) reconcileLoop(ctx context.Context) {

	periodicReconcileInterval := time.Second * 5

	// Use FS notifications to more quickly react to config file changes.
	var fsEvents chan fsnotify.Event
	var fsErrors chan error
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		// it's fine if we can't watch fs events, we will just periodically
		// look for config files changes..
		cntlr.logger.Warnf("Failed to watch FS notifications: %v", err)
		fsEvents = make(chan fsnotify.Event)
		fsErrors = make(chan error)
	} else {
		// since we can detect FS change events... we can increase the
		// periodic poll interval..
		periodicReconcileInterval = time.Second * 60
		fsEvents = watcher.Events
		defer watcher.Close()
	}

	reconcileTicker := time.NewTicker(periodicReconcileInterval)
	defer reconcileTicker.Stop()

	informerCtx, informerCancel := context.WithCancel(ctx)
	defer informerCancel()

	prefixes := []string{"tcp:", "udp:"}
	cntlr.metadataInformer = cntlr.client.DevicesApi.ListOrganizationMetadata(informerCtx, cntlr.org.Id, prefixes).Informer()

	configFile := filepath.Clean(cntlr.configFile)
	configDir, _ := filepath.Split(configFile)
	realConfigFile, _ := filepath.EvalSymlinks(cntlr.configFile)

	cntlr.lastErrorMsg = "none"
	for {

		if watcher != nil && len(watcher.WatchList()) == 0 {
			err = watcher.Add(configDir)
			if err != nil {
				cntlr.logger.Warn("Cannot watch '%s': %v", configDir, err)
			}
		}

		select {
		case <-ctx.Done():
			return

		case <-cntlr.informer.Changed():
			// in case of peer changes...
			cntlr.handleReconcile()

			// TODO: we should also reconcile when peer health changes.

		case <-cntlr.metadataInformer.Changed():
			// in case of metadata changes...
			cntlr.handleReconcile()

		case <-reconcileTicker.C:
			cntlr.handleReconcile()

		case err := <-fsErrors:
			cntlr.logger.Infof("fsnotify error: %v", err)
			for _, n := range watcher.WatchList() {
				_ = watcher.Remove(n)
			}
		case event := <-fsEvents:

			// inspired from from: https://github.com/spf13/viper/blob/e0f7631cf3ac7e7530949c7e154855076b0a4c17/viper.go

			// we only care about the config file with the following cases:
			// 1 - if the config file was modified or created
			eventFile := filepath.Clean(event.Name)
			configFileChanged := (eventFile == configFile && (event.Op.Has(fsnotify.Write) || event.Op.Has(fsnotify.Create)))

			// 2 - if the real path to the config file changed (eg: k8s ConfigMap replacement)
			currentConfigFile, _ := filepath.EvalSymlinks(cntlr.configFile)
			symLinksChanged := (currentConfigFile != "" && currentConfigFile != realConfigFile)

			if symLinksChanged || configFileChanged {
				realConfigFile = currentConfigFile
				cntlr.logger.Info("Triggering reconcile due to config file change")
				cntlr.handleReconcile()
			}
		}
	}
}

func (cntlr *SkupperConfigController) handleReconcile() {
	errorMsg := ""
	err := cntlr.reconcile()
	if err != nil {
		errorMsg = err.Error()
		// avoid spamming the log with the same error over and over... only log when the error changes.
		if errorMsg != cntlr.lastErrorMsg {
			cntlr.logger.Infow("skupper proxy configuration reconcile failed", "error", err)
		}
	} else {
		if cntlr.lastErrorMsg != "" {
			cntlr.logger.Info("skupper proxy configuration reconcile succeeded")
		}
	}
	cntlr.lastErrorMsg = errorMsg
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

	activeIngressPorts := map[HostPort]struct{}{}
	for _, rule := range desiredRules {

		// Keep track of which ingress ports are in use...
		if rule.ruleType == ProxyTypeIngress {
			activeIngressPorts[HostPort{
				host: string(rule.protocol),
				port: rule.listenPort,
			}] = struct{}{}
		}

		if _, found := diffSet[rule]; found {
			// take it out, anything remaining in the diffSet,
			// will be rules that need to be removed.
			delete(diffSet, rule)
		} else {
			// add the rule..
			proxy, err := cntlr.UserspaceProxyAdd(rule)
			if err != nil {
				return err
			}
			proxy.Start(cntlr.nexCtx, cntlr.nexWg, cntlr.userspaceNet)
		}
	}

	for rule := range diffSet {
		// remove the rule
		proxy, err := cntlr.UserspaceProxyRemove(rule)
		if err != nil {
			return err
		}
		proxy.Stop()

		if rule.ruleType == ProxyTypeIngress {
			protocol := string(rule.protocol)
			protocolPort := HostPort{
				host: protocol,
				port: rule.listenPort,
			}

			// if the allocated port is no longer used, then release it...
			if _, found := activeIngressPorts[protocolPort]; !found {
				address := cntlr.ingressPortToAddress[protocolPort]

				// release the port on the apiserver...
				_, err = cntlr.Nexodus.client.DevicesApi.
					DeleteDeviceMetadataKey(cntlr.nexCtx, cntlr.deviceId, protocolPort.String()).
					Execute()
				if err != nil {
					return fmt.Errorf("failed to update api server with port release: %w", err)
				}

				delete(cntlr.ingressPortToAddress, protocolPort)
				delete(cntlr.ingressAddressToPort, AddressKey{
					protocol: protocol,
					address:  address,
				})
			}
		}

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

		listenPort, err := cntlr.allocateIngressPortForAddress("tcp", endpoint.Address)
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

		destinations, err := cntlr.getServiceDestinations("tcp", endpoint.Address)
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

func (cntlr *SkupperConfigController) allocateIngressPortForAddress(protocol, address string) (int, error) {

	// is this the first time we are allocating a port?
	if cntlr.ingressPortToAddress == nil {

		// Get our previous port mapping from the API server...
		mds, _, err := cntlr.metadataInformer.Execute()
		if err != nil {
			return 0, err
		}

		ingressPortToAddress := map[HostPort]string{}
		ingressAddressToPort := map[AddressKey]int{}
		for _, md := range mds {
			if md.DeviceId != cntlr.Nexodus.deviceId {
				continue
			}

			protocolAndPort, err := parseHostPort(md.Key)
			if err != nil {
				return 0, err
			}

			mdAddress := asString(md.Value["address"])
			ingressPortToAddress[protocolAndPort] = mdAddress

			ingressAddressToPort[AddressKey{
				protocol: protocolAndPort.host,
				address:  mdAddress,
			}] = protocolAndPort.port
		}

		cntlr.ingressPortToAddress = ingressPortToAddress
		cntlr.ingressAddressToPort = ingressAddressToPort
		cntlr.nextPort = lowPortNumber
	}

	// was a port previously allocated for that protocol/address pair?
	port, found := cntlr.ingressAddressToPort[AddressKey{
		protocol: protocol,
		address:  address,
	}]
	if found {
		return port, nil
	}

	// find a free port to allocate.
	searchStartedAt := cntlr.nextPort
	startCounter := 0
	for { // loop until port set to value that is not allocated...

		port = cntlr.nextPort
		if searchStartedAt == port {
			startCounter += 1
			if startCounter > 1 { // avoid looping forever...
				return 0, fmt.Errorf("all ports are allocated")
			}
		}

		_, found := cntlr.ingressPortToAddress[HostPort{
			host: protocol,
			port: port,
		}]
		if !found {
			break
		}

		// try the next port....
		cntlr.nextPort += 1
		// we may need to wrap our search
		if cntlr.nextPort > highPortNumber {
			cntlr.nextPort = lowPortNumber
		}
		continue
	}

	protocolPort := HostPort{
		host: protocol,
		port: port,
	}

	// Post the port allocation to the API server...
	_, _, err := cntlr.Nexodus.client.DevicesApi.
		UpdateDeviceMetadataKey(cntlr.nexCtx, cntlr.deviceId, protocolPort.String()).
		Value(map[string]interface{}{"address": address}).
		Execute()
	if err != nil {
		return 0, fmt.Errorf("failed to update api server with port allocation: %w", err)
	}

	// and update our local port mapping cache... given we
	// are the only writer of the metadata to the api server,
	// we should not have to worry about the API server changing these values.
	cntlr.ingressPortToAddress[protocolPort] = address
	cntlr.ingressAddressToPort[AddressKey{
		protocol: protocol,
		address:  address,
	}] = port

	return port, nil
}

// getServiceDestinations find the ip:port address for devices which have posted
// metadata stating they are hosting the given service address and protocol
func (cntlr *SkupperConfigController) getServiceDestinations(protocol, serviceAddress string) ([]HostPort, error) {
	var result []HostPort

	// as the number of devices and services hosted by those devices grow,
	// the processing time of this function will grow...
	// TODO later: figure out how to reduce the work done by this function.

	// index devices by device_id
	devices := map[string]deviceCacheEntry{}
	cntlr.deviceCacheIterRead(func(entry deviceCacheEntry) {
		devices[entry.device.Id] = entry
	})

	mds, _, err := cntlr.metadataInformer.Execute()
	if err != nil {
		return result, err
	}

	for _, md := range mds {

		mdAddress := asString(md.Value["address"])
		if serviceAddress != mdAddress {
			continue
		}
		if !strings.HasPrefix(md.Key, protocol) {
			continue
		}

		// don't target destination on the local device..
		if md.DeviceId == cntlr.deviceId {
			continue
		}

		device, found := devices[md.DeviceId]
		if !found {
			continue
		}

		// don't target unhealthy devices
		if !device.peerHealthy {
			continue
		}

		protocolAndPort, err := parseHostPort(md.Key)
		if err != nil {
			return nil, err
		}

		result = append(result, HostPort{
			host: device.device.TunnelIp,
			port: protocolAndPort.port,
		})
	}

	// get all the address/port mappings for all node peers and find which nodes
	// are hosing the addresses
	return result, nil
}

func asString(value interface{}) string {
	switch value := value.(type) {
	case string:
		return value
	case nil:
		return ""
	case fmt.Stringer:
		return value.String()
	}
	return fmt.Sprintf("%v", value)
}

func (cntlr *SkupperConfigController) listProxyRules() ([]ProxyRule, error) {
	var result []ProxyRule
	cntlr.proxyLock.RLock()
	defer cntlr.proxyLock.RUnlock()
	for _, proxy := range cntlr.proxies {
		proxy.mu.RLock()
		result = append(result, proxy.rules...)
		proxy.mu.RUnlock()
	}
	return result, nil
}
