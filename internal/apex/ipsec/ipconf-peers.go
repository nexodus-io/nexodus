package ipsec

import (
	"fmt"
	"os"
	"text/template"
)

const (
	IpsecAuth    = "psk"
	IpsecAuthBy  = "secret"
	IpsecConf    = "/etc/swanctl/conf.d/swanctl.conf"
	IpsecSecrets = "/etc/swanctl/conf.d/swanctl-secrets.conf"
)

type ConfigIPSecTmpl struct {
	RelayIpsecAuth        string
	RelayReflexiveAddress string
	RelayNodeName         string
	LocalNodeName         string
	ConfigPeersTmpl       []PeerIPSecNodeTmpl
}

type PeerIPSecNodeTmpl struct {
	IpsecAuth     string
	IpsecAuthBy   string
	LocalNodeName string
	LocalCIDRs    string
	PeerNodeName  string
	PeerCIDRs     string
	RelayNodeName string
}

func NewPeerTmpl(localNodeName string, localCIDRs, peerNodeName, peerCIDRs, relayNodeName string) PeerIPSecNodeTmpl {
	peer := PeerIPSecNodeTmpl{
		IpsecAuth:     IpsecAuth,
		IpsecAuthBy:   IpsecAuthBy,
		LocalNodeName: localNodeName,
		LocalCIDRs:    localCIDRs,
		PeerNodeName:  peerNodeName,
		PeerCIDRs:     peerCIDRs,
		RelayNodeName: relayNodeName,
	}
	return peer
}

func BuildIPSecPeerTmpl(ipsecConfig ConfigIPSecTmpl) error {
	f, err := os.Create(IpsecConf)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", IpsecConf, err)
	}

	t := template.Must(template.New("ipsec_conf").Parse(ipsecConfTemplate))
	if err = t.Execute(f, ipsecConfig); err != nil {
		return fmt.Errorf("failed to fill template %s: %w", IpsecConf, err)
	}

	return nil
}

var ipsecConfTemplate = `

connections {
	{{ .RelayNodeName }} {
		mediation=yes
		remote_addrs={{ .RelayReflexiveAddress }}
		children {
			{{ .RelayNodeName }} {
				start_action=start
			}

		}
		local {
			auth = {{ .RelayIpsecAuth }}
			id = {{ .LocalNodeName }}
		}
		remote {
			auth = {{ .RelayIpsecAuth }}
			id = {{ .RelayNodeName }}
		}
        version = 2
	}

{{range $peer := .ConfigPeersTmpl}}
	{{ $peer.PeerNodeName }} {
		remote_addrs=%any
		children {
			peer {
				start_action=start
				local_ts={{ $peer.LocalCIDRs }}
				remote_ts={{ $peer.PeerCIDRs }}
			}

		}
		local {
			auth = {{ $peer.IpsecAuth }}
			id = {{ $peer.LocalNodeName }}
		}
		remote {
			auth = {{ $peer.IpsecAuth }}
			id = {{ $peer.PeerNodeName }}
		}
        version = 2
        mobike = no
        mediated_by = {{ .RelayNodeName }}
        mediation_peer = {{ $peer.PeerNodeName }}
	}
{{end}}
}

include /etc/swanctl/conf.d/swanctl-secrets.conf
`
