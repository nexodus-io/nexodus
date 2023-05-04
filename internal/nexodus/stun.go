package nexodus

import (
	_ "embed"
	"math/rand"
	"strings"
)

//go:embed stun-servers.txt
var stunServersTxtFile string

var (
	stunServer1 = ""
	stunServer2 = ""
	stunServers = []string{}
)

func init() {
	stunServers = strings.Split(stunServersTxtFile, "\n")
	for i := range stunServers {
		stunServers[i] = strings.TrimSpace(stunServers[i])
	}
	rand.Shuffle(len(stunServers), func(i, j int) {
		stunServers[i], stunServers[j] = stunServers[j], stunServers[i]
	})
	stunServer1 = stunServers[0]
	stunServer2 = stunServers[0]
}
