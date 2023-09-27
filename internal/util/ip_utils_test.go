package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsIPv4Address tests the IsIPv4Address function.
func TestIsIPv4Address(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		{"Valid IPv4 address", "100.64.0.1", true},
		{"Valid IPv6 address", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"Invalid IP address", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPv4Address(tt.addr)
			if result != tt.expected {
				t.Errorf("IsIPv4Address() got = %v, want = %v", result, tt.expected)
			}
		})
	}
}

// TestIsIPv6Address tests the IsIPv6Address function.
func TestIsIPv6Address(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected bool
	}{
		{"Valid IPv4 address", "100.64.0.1", false},
		{"Valid IPv6 address", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"Invalid IP address", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPv6Address(tt.addr)
			if result != tt.expected {
				t.Errorf("IsIPv6Address() got = %v, want = %v", result, tt.expected)
			}
		})
	}
}

// TestIsIPv4Prefix tests the IsIPv4Prefix function.
func TestIsIPv4Prefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		expected bool
	}{
		{"Valid IPv4 prefix", "192.168.1.0/24", true},
		{"Valid IPv6 prefix", "2001:db8::/32", false},
		{"Invalid prefix", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPv4Prefix(tt.prefix)
			if result != tt.expected {
				t.Errorf("IsIPv4Prefix() got = %v, want = %v", result, tt.expected)
			}
		})
	}
}

// TestIsIPv6Prefix tests the IsIPv6Prefix function.
func TestIsIPv6Prefix(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		expected bool
	}{
		{"Valid IPv4 prefix", "192.168.1.0/24", false},
		{"Valid IPv6 prefix", "2001:db8::/32", true},
		{"Invalid prefix", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPv6Prefix(tt.prefix)
			if result != tt.expected {
				t.Errorf("IsIPv6Prefix() got = %v, want = %v", result, tt.expected)
			}
		})
	}
}

// TestAppendPrefixMask tests the AppendPrefixMask function.
func TestAppendPrefixMask(t *testing.T) {
	tests := []struct {
		name        string
		ip          string
		maskSize    int
		expected    string
		expectError bool
	}{
		{"Valid IPv4 address and mask", "100.64.0.1", 24, "100.64.0.1/24", false},
		{"Valid IPv6 address and mask", "2001:db8::1", 32, "2001:db8::1/32", false},
		{"Invalid IP address", "invalid", 24, "", true},
		{"Invalid mask size for IPv4", "100.64.0.1", 33, "", true},
		{"Invalid mask size for IPv6", "2001:db8::1", 129, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AppendPrefixMask(tt.ip, tt.maskSize)
			if (err != nil) != tt.expectError {
				t.Errorf("AppendPrefixMask() error = %v, wantErr %v", err, tt.expectError)
				return
			}
			if result != tt.expected {
				t.Errorf("AppendPrefixMask() got = %v, want = %v", result, tt.expected)
			}
		})
	}
}

// TestIsIPv4DefaultRoute tests the IsIPv4DefaultRoute function.
func TestIsDefaultIPv4Route(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"0.0.0.0", true},
		{"0.0.0.0/0", true},
		{"192.168.1.1", false},
		{"192.168.1.1/24", false},
		{"invalid-default-route", false},
	}

	for _, tt := range testCases {
		t.Run(tt.input, func(t *testing.T) {
			result := IsDefaultIPv4Route(tt.input)
			assert.Equal(t, tt.expected, result, "isDefaultIPv4Route(%q) should be %t", tt.input, tt.expected)
		})
	}
}

// TestIsIPv6DefaultRoute tests the IsIPv6DefaultRoute function.
func TestIsDefaultIPv6Route(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"::", true},
		{"::/0", true},
		{"2001::1", false},
		{"2001::1/64", false},
		{"invalid-default-route", false},
	}

	for _, tt := range testCases {
		t.Run(tt.input, func(t *testing.T) {
			result := IsDefaultIPv6Route(tt.input)
			assert.Equal(t, tt.expected, result, "isDefaultIPv6Route(%q) should be %t", tt.input, tt.expected)
		})
	}
}

func TestContainsValidCustomIPv4Ranges(t *testing.T) {
	assert := assert.New(t)
	// Test CIDR notation
	assert.True(ContainsValidCustomIPv4Ranges([]string{"192.168.1.0/24"}))
	// Test individual IP
	assert.True(ContainsValidCustomIPv4Ranges([]string{"192.168.1.1"}))
	// Test Dash-separated range
	assert.True(ContainsValidCustomIPv4Ranges([]string{"192.168.1.1-192.168.1.2"}))
	// Test invalid ranges
	assert.False(ContainsValidCustomIPv4Ranges([]string{"192.300.1.1"}))
	assert.False(ContainsValidCustomIPv4Ranges([]string{"kitten_loaf"}))
}

func TestContainsValidCustomIPv6Ranges(t *testing.T) {
	assert := assert.New(t)
	// Test CIDR notation
	assert.True(ContainsValidCustomIPv6Ranges([]string{"2001:db8::/32"}))
	// Test individual IP
	assert.True(ContainsValidCustomIPv6Ranges([]string{"2001:db8::1"}))
	// Test Dash-separated range
	assert.True(ContainsValidCustomIPv6Ranges([]string{"2001:db8::1-2001:db8::2"}))
	// Test invalid ranges
	assert.False(ContainsValidCustomIPv6Ranges([]string{"making_biscuits"}))
	assert.False(ContainsValidCustomIPv6Ranges([]string{"2001:db8::zzz"}))
}
