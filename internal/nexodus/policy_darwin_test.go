//go:build darwin

package nexodus

import (
	"encoding/json"
	"testing"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/util"
	"github.com/stretchr/testify/assert"
)

func runTestPacketFilterRuleBuilder(t *testing.T, securityGroupJSON string, expectedRules []string) {
	var secGroup public.ModelsSecurityGroup
	err := json.Unmarshal([]byte(securityGroupJSON), &secGroup)
	if err != nil {
		t.Fatalf("Error unmarshaling JSON: %v", err)
	}

	// Initialize pfRuleBuilder
	prb := &pfRuleBuilder{iface: "utun8"}

	// Explicit drop if inbound rules are defined
	if len(secGroup.InboundRules) > 0 {
		prb.pfBlockAll("in")
	}
	// Process inbound rules
	for _, rule := range secGroup.InboundRules {
		if len(rule.IpRanges) == 0 || containsEmptyRange(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "inbound"); err != nil {
				t.Errorf("pfctl setup error, failed to process inbound rule with 'any': %v", err)
			}
		} else if util.ContainsValidCustomIPv4Ranges(rule.IpRanges) || util.ContainsValidCustomIPv6Ranges(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAddr(rule, "inbound"); err != nil {
				t.Errorf("pfctl setup error, failed to process inbound rule: %v", err)
			}
		} else {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "inbound"); err != nil {
				t.Errorf("pfctl setup error, failed to process inbound rule with 'any': %v", err)
			}
		}
	}
	// Explicit drop if outbound rules are defined
	if len(secGroup.OutboundRules) > 0 {
		prb.pfBlockAll("out")
	}
	// Process outbound rules
	for _, rule := range secGroup.OutboundRules {
		if len(rule.IpRanges) == 0 || containsEmptyRange(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "outbound"); err != nil {
				t.Errorf("pfctl setup error, failed to process outbound rule with 'any': %v", err)
			}
		} else if util.ContainsValidCustomIPv4Ranges(rule.IpRanges) || util.ContainsValidCustomIPv6Ranges(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAddr(rule, "outbound"); err != nil {
				t.Errorf("pfctl setup error, failed to process outbound rule: %v", err)
			}
		} else {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "outbound"); err != nil {
				t.Errorf("pfctl setup error, failed to process outbound rule with 'any': %v", err)
			}
		}
	}

	// Assert and output the generated rules for debugging
	t.Logf("Generated pf rules:\n%s\n", prb.sb.String())

	for _, rule := range expectedRules {
		assert.Contains(t, prb.sb.String(), rule, "Generated rules should include: "+rule)
	}
}

func TestDarwinRuleBuilder(t *testing.T) {
	mockSecurityGroup1 := `
{
	"group_name": "Test",
	"inbound_rules": [
		{"ip_protocol": "ipv4", "ip_ranges": ["10.0.0.1/24", "192.168.1.1/32"]},
		{"ip_protocol": "ipv4", "ip_ranges": ["0.0.0.0/0"]},
		{"ip_protocol": "tcp"},
		{"ip_protocol": "ipv6", "ip_ranges": ["::1/128", "2001:db8::/64"]},
		{"from_port": 22, "to_port": 22, "ip_protocol": "tcp", "ip_ranges": ["10.0.0.1", "192.168.0.1"]},
		{"from_port": 80, "to_port": 80, "ip_protocol": "tcp", "ip_ranges": ["10.0.0.2", "10.0.0.3"]},
		{"ip_protocol": "icmpv4", "ip_ranges": ["10.0.0.1"]},
		{"ip_protocol": "icmpv4"},
		{"ip_protocol": "tcp", "from_port": 9001, "to_port": 9005, "ip_ranges": ["100.64.0.1 - 100.64.0.45"]},
		{"ip_protocol": "icmpv6", "ip_ranges": ["::1"]},
		{"ip_protocol": "icmp", "ip_ranges": ["::1"]},
		{"ip_protocol": "udp"},
		{"ip_protocol": "tcp", "from_port": 22, "to_port": 22},
		{"ip_protocol": "icmp6"},
		{"ip_protocol": "icmp4"},
		{"ip_protocol": "ipv4"},
		{"ip_protocol": "udp", "from_port": 54, "to_port": 54, "ip_ranges": ["100.64.0.1 - 100.64.0.45", "100.64.100.0/24", "100.64.0.120"]},
		{"ip_protocol": "ipv4", "from_port": 9000, "to_port": 9001},
		{"ip_protocol": "ipv6", "from_port": 43333, "to_port": 53333},
		{"ip_protocol": "ipv6", "from_port": 0, "to_port": 0},
		{"ip_protocol": "ipv4", "from_port": 0, "to_port": 0, "ip_ranges": [""]}
	],
	"outbound_rules": [
		{"ip_protocol": "tcp", "from_port": 80, "to_port": 80, "ip_ranges": ["192.168.0.1", "10.120.0.2"]},
		{"ip_protocol": "udp", "from_port": 53, "to_port": 53, "ip_ranges": ["8.8.8.8"]},
		{"ip_protocol": "icmpv6", "ip_ranges": ["::1", "2001:db8:1234::/48"]},
		{"ip_protocol": "icmp6"},
		{"ip_protocol": "icmp"},
		{"ip_protocol": "ipv4", "ip_ranges": ["", "10.0.0.3"]},
		{"ip_protocol": "ipv4", "from_port": 123, "to_port": 456},
		{"ip_protocol": "ipv6", "from_port": 41000, "to_port": 41500},
		{"ip_protocol": "ipv4", "from_port": 52000, "to_port": 53000, "ip_ranges": ["192.168.0.1", "10.130.0.2"]},
		{"ip_protocol": "ipv6", "from_port": 78, "to_port": 89, "ip_ranges": ["2001:db9::2/64", "3001:da9::2-3001:da9::6"]},
		{"ip_protocol": "icmp", "from_port": 0, "to_port": 0, "ip_ranges": [""]}
	]
}
`

	mockSecurityGroup1ExpectedRules := []string{
		"block in on utun8 all",
		"pass in quick on utun8 inet from { 10.0.0.1/24, 192.168.1.1/32 } to any",
		"pass in quick on utun8 inet from { 0.0.0.0/0 } to any",
		"pass in quick on utun8 inet proto tcp from any to any",
		"pass in quick on utun8 inet6 from { ::1/128, 2001:db8::/64 } to any",
		"pass in quick on utun8 inet proto tcp from { 10.0.0.1, 192.168.0.1 } to any port 22:22",
		"pass in quick on utun8 inet proto tcp from { 10.0.0.2, 10.0.0.3 } to any port 80:80",
		"pass in quick on utun8 inet proto icmp from { 10.0.0.1 } to any",
		"pass in quick on utun8 inet proto icmp from any to any",
		"pass in quick on utun8 inet proto tcp from { 100.64.0.1  -  100.64.0.45 } to any port 9001:9005",
		"pass in quick on utun8 inet6 proto icmp6 from { ::1 } to any",
		"pass in quick on utun8 inet proto icmp to any",
		"pass in quick on utun8 inet6 proto icmp6 to any",
		"pass in quick on utun8 inet proto udp from any to any",
		"pass in quick on utun8 inet proto tcp from any to any port 22:22",
		"pass in quick on utun8 inet6 proto icmp6 from any to any",
		"pass in quick on utun8 inet proto icmp from any to any",
		"pass in quick on utun8 inet from any to any",
		"pass in quick on utun8 inet proto udp from { 100.64.0.1  -  100.64.0.45, 100.64.100.0/24, 100.64.0.120 } to any port 54:54",
		"pass in quick on utun8 inet proto tcp from any to any port 9000:9001",
		"pass in quick on utun8 inet proto udp from any to any port 9000:9001",
		"pass in quick on utun8 inet6 proto tcp from any to any port 43333:53333",
		"pass in quick on utun8 inet6 proto udp from any to any port 43333:53333",
		"pass in quick on utun8 inet6 from any to any",
		"pass in quick on utun8 inet from any to any",
		"block out on utun8 all",
		"pass out quick on utun8 inet proto tcp to { 192.168.0.1, 10.120.0.2 } port 80:80",
		"pass out quick on utun8 inet proto udp to { 8.8.8.8 } port 53:53",
		"pass out quick on utun8 inet6 proto icmp6 to { ::1, 2001:db8:1234::/48 }",
		"pass out quick on utun8 inet6 proto icmp6 to any",
		"pass out quick on utun8 inet proto icmp to any",
		"pass out quick on utun8 inet6 proto icmp6 to any",
		"pass out quick on utun8 inet to any",
		"pass out quick on utun8 inet proto tcp to any port 123:456",
		"pass out quick on utun8 inet proto udp to any port 123:456",
		"pass out quick on utun8 inet6 proto tcp to any port 41000:41500",
		"pass out quick on utun8 inet6 proto udp to any port 41000:41500",
		"pass out quick on utun8 inet proto tcp to { 192.168.0.1, 10.130.0.2 } port 52000:53000",
		"pass out quick on utun8 inet proto udp to { 192.168.0.1, 10.130.0.2 } port 52000:53000",
		"pass out quick on utun8 inet6 proto tcp to { 2001:db9::2/64, 3001:da9::2 - 3001:da9::6 } port 78:89",
		"pass out quick on utun8 inet6 proto udp to { 2001:db9::2/64, 3001:da9::2 - 3001:da9::6 } port 78:89",
		"pass in quick on utun8 inet proto icmp from any to any",
		"pass in quick on utun8 inet6 proto icmp6 from any to any",
	}

	t.Run("Test with mockSecurityGroup1", func(t *testing.T) {
		runTestPacketFilterRuleBuilder(t, mockSecurityGroup1, mockSecurityGroup1ExpectedRules)
	})
}
