package stun

import (
	_ "embed"
	"math/rand"
	"strings"
	"sync"
)

//go:embed stun-servers.txt
var stunServersTxtFile string
var currentStunServer = 0
var stunServerMu = sync.Mutex{}

var (
	stunServers = []string{}
)

func init() {
	var servers []string
	for _, server := range strings.Split(stunServersTxtFile, "\n") {
		server = strings.TrimSpace(server)
		if server != "" {
			servers = append(servers, strings.TrimSpace(server))
		}
	}
	SetServers(servers)
}

func SetServers(servers []string) {
	stunServerMu.Lock()
	defer stunServerMu.Unlock()
	stunServers = servers
	// #nosec G404
	rand.Shuffle(len(stunServers), func(i, j int) {
		stunServers[i], stunServers[j] = stunServers[j], stunServers[i]
	})
	currentStunServer = 0
}

func NextServer() string {
	stunServerMu.Lock()
	defer stunServerMu.Unlock()
	currentStunServer += 1
	if currentStunServer >= len(stunServers) {
		currentStunServer = 0
	}
	return stunServers[currentStunServer]
}
