package nexodus

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
	stunServers = strings.Split(stunServersTxtFile, "\n")
	for i := range stunServers {
		stunServers[i] = strings.TrimSpace(stunServers[i])
	}
	// #nosec G404
	rand.Shuffle(len(stunServers), func(i, j int) {
		stunServers[i], stunServers[j] = stunServers[j], stunServers[i]
	})
}

func NextStunServer() string {
	stunServerMu.Lock()
	defer stunServerMu.Unlock()
	currentStunServer += 1
	if currentStunServer >= len(stunServers) {
		currentStunServer = 0
	}
	return stunServers[currentStunServer]
}
