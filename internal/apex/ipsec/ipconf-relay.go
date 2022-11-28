package ipsec

import (
	"fmt"
	"os"
	"text/template"
)

type RelayIPSecNodeTmpl struct {
	IpsecAuth        string
	IpsecAuthBy      string
	LocalNodeAddress string
	RelayNodeName    string
}

func NewRelayTmpl(localNodeAddress, relayNodeName string) RelayIPSecNodeTmpl {
	relay := RelayIPSecNodeTmpl{
		IpsecAuth:        IpsecAuth,
		IpsecAuthBy:      IpsecAuthBy,
		LocalNodeAddress: localNodeAddress,
		RelayNodeName:    relayNodeName,
	}
	return relay
}

func BuildRelayIPSecConfFile(relay RelayIPSecNodeTmpl) error {
	f, err := os.Create(IpsecConf)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", IpsecConf, err)
	}

	t := template.Must(template.New("ipsec_conf").Parse(ipsecRelayTemplate))
	if err = t.Execute(f, relay); err != nil {
		return fmt.Errorf("failed to fill template %s: %w", IpsecConf, err)
	}

	return nil
}

var ipsecRelayTemplate = `
connections {
   {{ .RelayNodeName }} {
      local_addrs  = {{ .LocalNodeAddress }}
      local {
         auth = {{ .IpsecAuth }}
         id = {{ .RelayNodeName }}
      }
      remote {
         auth = psk
      }
      version = 2
      mobike = no
      mediation = yes
   }
}

include /etc/swanctl/conf.d/swanctl-secrets.conf
`
