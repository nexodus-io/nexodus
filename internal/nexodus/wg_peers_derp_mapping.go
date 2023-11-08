package nexodus

import (
	"fmt"
	"net"
	"sync"
)

// DerpIpMapping represents the mapping between private keys and IP addresses.
type DerpIpMapping struct {
	mapping         map[string]string
	lastAllocatedIp net.IP
	mutex           sync.Mutex
}

// NewIPMapping creates a new instance of IPMapping.
func NewDerpIpMapping() *DerpIpMapping {
	return &DerpIpMapping{
		mapping:         make(map[string]string),
		lastAllocatedIp: net.IPv4(127, 0, 0, 1),
	}
}

// GetIPAddress retrieves the public key associated with a given ip address.
func (dim *DerpIpMapping) GetPublicKey(ipAddress string) (string, bool) {
	dim.mutex.Lock()
	defer dim.mutex.Unlock()
	ip, exists := dim.mapping[ipAddress]
	return ip, exists
}

// GetLocalIPMappingForPeer finds the next available IP address in the 127.0.0.0/24 range.
func (dim *DerpIpMapping) GetLocalIPMappingForPeer(publicKey string) (string, error) {
        if ip := dim.CheckIfKeyExist(publicKey) ; ip != "" {
                return ip, nil
        }
	dim.mutex.Lock()
	defer dim.mutex.Unlock()

	// Start from 127.0.0.1 and find the next available IP address
	// TODO: Fix this for 127.0.0.0/8 range
	baseIP := dim.lastAllocatedIp.To4()
	for i := 1; i <= 254-int(baseIP[3]); i++ {
		ip := net.IPv4(baseIP[0], baseIP[1], baseIP[2], baseIP[2]+byte(i))
		ipString := ip.String()
		if _, exists := dim.mapping[ipString]; !exists {
			dim.lastAllocatedIp = ip
			dim.mapping[ipString] = publicKey
			return ipString, nil
		}
	}

	return "", fmt.Errorf("no available IP addresses in the 127.0.0.0/24 range")
}

func (dim *DerpIpMapping) RemoveLocalIpMappingForPeer(publicKey string) error {
        dim.mutex.Lock()
        defer dim.mutex.Unlock()

        for ip, pk := range dim.mapping {
                if pk == publicKey {
                        delete(dim.mapping, ip)
                        return nil
                }
        }

        return fmt.Errorf("no IP address found for the given public key")
}

func (dim *DerpIpMapping) CheckIfKeyExist(publicKey string) string {
        dim.mutex.Lock()
        defer dim.mutex.Unlock()

        for ip, pk := range dim.mapping {
                if pk == publicKey {
                        return ip
                }
        }

        return ""
}