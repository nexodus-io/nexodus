package state

import (
	"fmt"
	"golang.org/x/oauth2"
)

type State struct {
	AuthToken        *oauth2.Token    `json:"auth-token,omitempty"`
	PublicKey        string           `json:"public-key"`
	PrivateKey       string           `json:"private-key"`
	ProxyRulesConfig ProxyRulesConfig `json:"proxy-rules-config"`
}

type ProxyRulesConfig struct {
	Egress  []string `json:"egress"`
	Ingress []string `json:"ingress"`
}

type Store interface {
	fmt.Stringer
	Load() error
	Store() error
	State() *State
}
