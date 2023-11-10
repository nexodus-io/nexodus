package util

import (
	"errors"
	"fmt"
	"net"
	"strings"
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

// ValidateIPv6Cidr checks for a valid IPv6 cidr
func ValidateIPv6Cidr(cidrStr string) error {
	_, cidr, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return err
	}
	if cidr.IP.To16() == nil {
		return errors.New("not an ipv6 cidr")
	}
	return nil
}

// ValidateIPv4Cidr checks for a valid IPv4 cidr
func ValidateIPv4Cidr(cidrStr string) error {
	_, cidr, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return err
	}
	if cidr.IP.To4() == nil {
		return errors.New("not an ipv4 cidr")
	}
	return nil
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

// IsDefaultIPRoute wrapper for IsDefaultIPv4Route and IsDefaultIPv6Route
func IsDefaultIPRoute(ip string) bool {
	return IsDefaultIPv4Route(ip) || IsDefaultIPv6Route(ip)
}

// IsValidPrefix checks if the given cidr is valid
func IsValidPrefix(prefix string) bool {
	return IsIPv4Prefix(prefix) || IsIPv6Prefix(prefix)
}

// ContainsValidCustomIPv4Ranges matches the following custom IPv4 patterns usable by netfilter userspace utils:
// Cidr notation 100.100.0.0/16
// Individual address 10.100.0.2
// Dash-separated range 100.100.0.0-100.100.10.255
func ContainsValidCustomIPv4Ranges(ipRanges []string) bool {
	for _, ipRange := range ipRanges {
		if strings.Contains(ipRange, "-") {
			// Dash-separated range
			ips := strings.Split(ipRange, "-")
			ip1 := net.ParseIP(strings.TrimSpace(ips[0]))
			ip2 := net.ParseIP(strings.TrimSpace(ips[1]))

			if ip1 != nil && ip1.To4() != nil && ip2 != nil && ip2.To4() != nil {
				return true
			}
		} else if strings.Contains(ipRange, "/") {
			// CIDR notation
			_, ipNet, err := net.ParseCIDR(ipRange)
			if err == nil && ipNet.IP.To4() != nil {
				return true
			}
		} else {
			ip := net.ParseIP(ipRange)
			// Individual IP
			if ip != nil && ip.To4() != nil {
				return true
			}
		}
	}

	return false
}

// ContainsValidCustomIPv6Ranges matches the following custom IPv6 patterns usable by netfilter userspace utils:
// Cidr notation 200::/64
// Individual address 200::2
// Dash-separated range 200::1-200::8
// Dash-separated range 2001:0db8:0000:0000:0000:0000:0000:0000-2001:0db8:ffff:ffff:ffff:ffff:ffff:ffff
func ContainsValidCustomIPv6Ranges(ipRanges []string) bool {
	for _, ipRange := range ipRanges {
		if strings.Contains(ipRange, "-") {
			// Dash-separated range
			ips := strings.Split(ipRange, "-")
			if len(ips) != 2 {
				return false
			}

			ip1 := net.ParseIP(strings.TrimSpace(ips[0]))
			ip2 := net.ParseIP(strings.TrimSpace(ips[1]))

			if ip1 == nil || ip2 == nil || ip1.To16() == nil || ip2.To16() == nil {
				return false
			}
		} else if strings.Contains(ipRange, "/") {
			// CIDR notation
			_, _, err := net.ParseCIDR(ipRange)
			if err != nil {
				return false
			}
		} else {
			// Individual IP
			ip := net.ParseIP(ipRange)
			if ip == nil || ip.To16() == nil {
				return false
			}
		}
	}

	return true
}
