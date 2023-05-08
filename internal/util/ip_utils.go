package util

import (
	"fmt"
	"net"
)

// IsIPv4Address checks if the given IP address is an IPv4 address.
func IsIPv4Address(addr string) bool {
	ip := net.ParseIP(addr)
	return ip.To4() != nil
}

// IsIPv6Address checks if the given IP address is an IPv6 address.
func IsIPv6Address(addr string) bool {
	ip := net.ParseIP(addr)
	return ip != nil && ip.To4() == nil
}

// IsIPv4Prefix checks if the given IP address is an IPv4 prefix.
func IsIPv4Prefix(prefix string) bool {
	_, ipv4Net, err := net.ParseCIDR(prefix)
	if err != nil {
		return false
	}
	return ipv4Net.IP.To4() != nil
}

// IsIPv6Prefix checks if the given IP address is an IPv6 prefix.
func IsIPv6Prefix(prefix string) bool {
	_, ipv6Net, err := net.ParseCIDR(prefix)
	if err != nil {
		return false
	}
	return ipv6Net.IP.To4() == nil
}

// AppendPrefixMask appends a prefix mask to a v4 or v6 address and returns the result as a string.
func AppendPrefixMask(ipStr string, maskSize int) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipStr)
	}

	ipLen := net.IPv4len
	if ip.To4() == nil {
		ipLen = net.IPv6len
	}

	if maskSize > ipLen*8 {
		return "", fmt.Errorf("invalid mask size for IP address %s: %d", ip.String(), maskSize)
	}

	mask := net.CIDRMask(maskSize, ipLen*8)
	ipNet := net.IPNet{IP: ip, Mask: mask}

	return ipNet.String(), nil
}

// IsDefaultIPv4Route checks if the given prefix is the default route for IPv4.
func IsDefaultIPv4Route(ip string) bool {
	ipAddr, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		ipAddr = net.ParseIP(ip)
		if ipAddr == nil {
			return false
		}
	}

	ipv4Default := net.IPv4(0, 0, 0, 0)
	return ipAddr.Equal(ipv4Default) || (ipNet != nil && ipNet.IP.Equal(ipv4Default) && ipNet.Mask.String() == "0.0.0.0")
}

// IsDefaultIPv6Route checks if the given prefix is the default route for IPv6.
func IsDefaultIPv6Route(ip string) bool {
	ipAddr, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		ipAddr = net.ParseIP(ip)
		if ipAddr == nil {
			return false
		}
	}

	ipv6Default := net.ParseIP("::")
	return ipAddr.Equal(ipv6Default) || (ipNet != nil && ipNet.IP.Equal(ipv6Default) && ipNet.Mask.String() == "<nil>")
}
