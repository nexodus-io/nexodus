package apex

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

const (
	WgDefaultPort     = 51820
	WgLinuxConfPath   = "/etc/wireguard/"
	WgDarwinConfPath  = "/usr/local/etc/wireguard/"
	WgWindowsConfPath = "C:/"
	wgConfActive      = "wg0.conf"
	wgConfLatestRev   = "wg0-latest-rev.conf"
	wgIface           = "wg0"
	darwinIface       = "utun8"
)

func (ax *Apex) overwriteWgConfig() error {
	activeConfig := filepath.Join(WgLinuxConfPath, wgConfActive)
	latestRevConfig := filepath.Join(WgLinuxConfPath, wgConfLatestRev)
	// copy the latest config rev to wg0.conf
	if err := CopyFile(latestRevConfig, activeConfig); err != nil {
		return err
	}
	if err := wgConfPermissions(activeConfig); err != nil {
		return fmt.Errorf("failed to start the wireguard config permissions: %v", err)
	}
	return nil
}

// updateWireguardConfig strip and diff the latest rev to the active config
// TODO: use syncconf and manually track routes instead of wg-quick managing them?
func (ax *Apex) updateWireguardConfig() error {
	activeConfig := filepath.Join(WgLinuxConfPath, wgConfActive)
	latestRevConfig := filepath.Join(WgLinuxConfPath, wgConfLatestRev)
	// If no config exists, copy the latest rev config to /etc/wireguard/wg0-latest-rev.conf
	if _, err := os.Stat(activeConfig); err != nil {
		latestConfig := filepath.Join(WgLinuxConfPath, wgConfLatestRev)
		if err := CopyFile(latestConfig, activeConfig); err == nil {
			return nil
		} else {
			return err
		}
	}
	// If a config exists, strip and diff the active and latest revision
	stripActiveCfg, err := RunCommand("wg-quick", "strip", activeConfig)
	if err != nil {
		return fmt.Errorf("failed to strip the active configuration: %v", err)
	}
	stripLatestRevCfg, err := RunCommand("wg-quick", "strip", latestRevConfig)
	if err != nil {
		return fmt.Errorf("failed to strip the latest configuration: %v", err)
	}
	// TODO: WARNING!!! THIS IS A HACK SINCE unmarshallWireguardCfg()
	// CANNOT HANDLE AN EMPTY [Peers] SECTION. THIS MATCHES [Peer.xxx]
	if !strings.Contains(stripActiveCfg, "[Peer") {
		err := ax.overwriteWgConfig()
		if err != nil {
			return err
		}
		return nil
	}
	// TODO: WARNING!!! THIS IS A HACK SINCE unmarshallWireguardCfg()
	// CANNOT HANDLE AN EMPTY [Peers] SECTION. THIS MATCHES [Peer.xxx]
	if !strings.Contains(stripLatestRevCfg, "[Peer") {
		log.Print("No peers found in the latest config")
		err := ax.overwriteWgConfig()
		if err != nil {
			return err
		}
		return nil
	}
	// unmarshall the active and latest configurations
	activePeers, err := unmarshallWireguardCfg(stripActiveCfg)
	if err != nil {
		return err
	}
	revisedPeers, err := unmarshallWireguardCfg(stripLatestRevCfg)
	if err != nil {
		return err
	}

	// diff the unordered list of [Peers] configuration peers and rebuild the tunnel if there has been a change
	if !diffWireguardConfigs(activePeers.Peers, revisedPeers.Peers) {
		log.Print("Peers configuration change detected")
		if err := ax.overwriteWgConfig(); err != nil {
			return err
		}
	}
	// if there is no wg0 interface, write the configuration, for mac just do it everytime
	if !linkExists(wgIface) {
		if err := ax.overwriteWgConfig(); err != nil {
			return err
		}
	}
	return nil
}

func unmarshallWireguardCfg(iniConfig string) (*wgConfig, error) {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment:     true,
		AllowNonUniqueSections:  true,
		SkipUnrecognizableLines: true,
	}, strings.NewReader(iniConfig))

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
