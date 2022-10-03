package main

import (
	"fmt"
	"gopkg.in/ini.v1"
	"strings"
)

func unmarshallWireguadCfg(iniConfig string) (*wgConfig, error) {
	iniFile, err := ini.LoadSources(ini.LoadOptions{AllowNonUniqueSections: true}, strings.NewReader(iniConfig))
	if err != nil {
		return nil, fmt.Errorf("error loading wireguard ini file: %v", err)
	}
	wg := &wgConfig{}
	if err := iniFile.MapTo(wg); err != nil {
		return nil, fmt.Errorf("error parsing wireguard ini file: %v", err)
	}
	return wg, nil
}

// diffWireguardConfigs compares the peer configurations (unordered lists)
func diffWireguardConfigs(activeCfg, revCfg []wgPeerConfig) bool {
	// check for a new or removed peer
	if len(activeCfg) != len(revCfg) {
		return false
	}
	xMap := make(map[string]int)
	yMap := make(map[string]int)
	// look for any endpoint changes in existing peers
	for _, xElem := range activeCfg {
		xMap[xElem.Endpoint]++
	}
	for _, yElem := range revCfg {
		yMap[yElem.Endpoint]++
	}
	for xEndpointKey, xKeyEndpointVal := range xMap {
		if yMap[xEndpointKey] != xKeyEndpointVal {
			return false
		}
	}
	// look for any public key changes in existing peers
	for _, xElem := range activeCfg {
		xMap[xElem.PublicKey]++
	}
	for _, yElem := range revCfg {
		yMap[yElem.PublicKey]++
	}
	for xKeyPublicKey, xKeyPublicVal := range xMap {
		if yMap[xKeyPublicKey] != xKeyPublicVal {
			return false
		}
	}
	return true
}
