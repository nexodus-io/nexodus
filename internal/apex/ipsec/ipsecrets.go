package ipsec

import (
	"fmt"
	"os"
	"text/template"
)

type PreSharedKeysTmpl struct {
	PreSharedKey map[string]string
}

func BuildSecretsFile(relay PreSharedKeysTmpl) error {
	f, err := os.Create(IpsecSecrets)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", IpsecSecrets, err)
	}

	t := template.Must(template.New("ipsec_psk").Parse(ipsecPskTemplate))
	if err = t.Execute(f, relay); err != nil {
		return fmt.Errorf("failed to fill template %s: %w", IpsecSecrets, err)
	}

	return nil
}

var ipsecPskTemplate = `
secrets {
{{range $id, $psk := .PreSharedKey}}
    ike-{{$id}} {
        id = {{$id}}
        secret = {{$psk}}
    }
{{end}}
}
`
