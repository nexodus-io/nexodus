package apex

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/apex/ipsec"
	log "github.com/sirupsen/logrus"
	"strings"
)

func (ax *Apex) BuildIPSecPeerConfig() {

	var err error
	var relayPublicIP string
	var relayNodeName string
	ax.updateKeyCache()

	// if this node is the relay node, return since it does not create individual peers
	if ax.hubRouter {
		return
	}
	// parse the relay node information for the mediator node
	for _, value := range ax.peerCache {
		// register the public address of the relay node to populate peer nodes
		if value.HubRouter {
			relayPublicIP = value.ReflexiveIPv4
			relayNodeName, err = ax.getDeviceHostname(value.DeviceID)
			if err != nil {
				log.Error(err)
			}
		}
	}
	// set the relay/mediation connection details
	ipsecConfig := ipsec.ConfigIPSecTmpl{}
	ipsecConfig.RelayNodeName = relayNodeName
	ipsecConfig.RelayReflexiveAddress = relayPublicIP
	ipsecConfig.RelayIpsecAuth = ipsec.IpsecAuth
	ipsecConfig.LocalNodeName = ax.hostname
	// range through all cached peers
	for _, value := range ax.peerCache {
		device, err := ax.client.GetDevice(value.DeviceID)
		if err != nil {
			log.Fatalf("unable to get device %s: %s", value.DeviceID, err)
		}
		// don't process the peer if it is the local node itself
		if device.PublicKey != ax.wireguardPubKey {
			// register left-side (local) ipsec details when the current value matches this node's public key
			localCIDRs := []string{ax.endpointLocalAddressIPv4}
			peerCIDRs := []string{value.EnpointLocalAddressIPv4}
			device, err := ax.client.GetDevice(value.DeviceID)
			if err != nil {
				log.Fatalf("unable to get device %s: %s", value.DeviceID, err)
			}
			peerNodeName := device.Hostname

			// right-side (remote) peer details for each remote connection from the remaining peers (if not a relay node)
			if !value.HubRouter {
				peer := ipsec.NewPeerTmpl(ipsecConfig.LocalNodeName, AllowedIPsString(localCIDRs), peerNodeName, AllowedIPsString(peerCIDRs), relayNodeName)
				ipsecConfig.ConfigPeersTmpl = append(ipsecConfig.ConfigPeersTmpl, peer)
			}
		}
	}

	// build and write to the config template file with new or updated peers
	if err := ipsec.BuildIPSecPeerTmpl(ipsecConfig); err != nil {
		log.Fatalf("error building the ipsec peer template: %v", err)
	}
}

// IpsecSecrets TODO: add support for certs.
func (ax *Apex) IpsecSecrets() {
	psk := ipsec.PreSharedKeysTmpl{
		PreSharedKey: make(map[string]string),
	}
	psk.PreSharedKey[ax.hostname] = ax.ikePSK
	// TODO: this might be redundant but needs to be tested against a large
	// number of onboards to see if the local key gets populated in time
	keys := FileToString(ipsec.IpsecSecrets)
	// add the local node's key if it hasn't been already
	if !strings.Contains(keys, ax.hostname) {
		if err := ipsec.BuildSecretsFile(psk); err != nil {
			log.Fatal(err)
		}
	}
	// no need to process peer keys if this node is the relay
	if ax.hubRouter {
		return
	}
	// process peer PSKs
	for _, value := range ax.peerCache {
		device, err := ax.client.GetDevice(value.DeviceID)
		if err != nil {
			log.Fatalf("unable to get device %s: %s", value.DeviceID, err)
		}
		// TODO: cache hostnames with keycache
		psk.PreSharedKey[device.Hostname] = ax.ikePSK
	}
	if err := ipsec.BuildSecretsFile(psk); err != nil {
		log.Fatal(err)
	}
}

func (ax *Apex) updateKeyCache() {
	for _, peer := range ax.peerCache {
		var ok bool
		if _, ok = ax.keyCache[peer.DeviceID]; !ok {
			device, err := ax.client.GetDevice(peer.DeviceID)
			if err != nil {
				log.Fatalf("unable to get device %s: %s", peer.DeviceID, err)
			}
			ax.keyCache[peer.DeviceID] = device.PublicKey
		}
	}
}

func (ax *Apex) getDeviceHostname(devID uuid.UUID) (string, error) {
	device, err := ax.client.GetDevice(devID)
	if err != nil {
		return "", fmt.Errorf("unable to get device %v: %s", device, err)
	}

	return device.Hostname, nil
}

// BuildIPSecRelayConfig build the mediation config file
func (ax *Apex) BuildIPSecRelayConfig() error {
	// if this node is the relay node, build the mediation config file and return since it does not create individual peers
	if ax.hubRouter {
		relayConfig := ipsec.NewRelayTmpl(ax.endpointLocalAddressIPv4, ax.hostname)
		// build and write to the config template file with new or updated peers
		if err := ipsec.BuildRelayIPSecConfFile(relayConfig); err != nil {
			return fmt.Errorf("error building the ipsec template: %v", err)
		}
	}
	ax.IpsecSecrets()

	return ipsec.RestartIPSecSystemd()
}
