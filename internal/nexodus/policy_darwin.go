//go:build darwin

package nexodus

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	basePFFile         = "/etc/pf.conf"
	pfAnchorFile       = "/etc/pf.anchors/io.nexodus"
	appleSharingAnchor = "com.apple.internet-sharing"
)

type pfRuleBuilder struct {
	sb     strings.Builder
	iface  string
	pfFile string
}

func (nx *Nexodus) processSecurityGroupRules() error {
	// Check if SecurityGroup is nil or has no rules, if any of the conditionals match, create an empty anchor
	// file permitting all traffic and return. The goal is to not interrupt any existing PF rules. If pfctl
	// is already running, we leave it alone and simply write an empty file permitting all traffic.
	// If pfctl is disabled on the host and there are no rules we leave it disabled.
	if nx.securityGroup == nil || (len(nx.securityGroup.InboundRules) == 0 && len(nx.securityGroup.OutboundRules) == 0) {
		if _, err := os.Stat(pfAnchorFile); os.IsNotExist(err) {
			// Create the file if it does not exist
			_, err := os.Create(pfAnchorFile)
			if err != nil {
				return fmt.Errorf("failed to create anchor file: %w", err)
			}
		}
		if err := writeEmptyRuleSet(pfAnchorFile); err != nil {
			return err
		}

		return nil
	}

	// Check and enable pfctl if not enabled
	if err := checkAndEnablePfctl(nx.logger); err != nil {
		return fmt.Errorf("failed to ensure pfctl is enabled: %w", err)
	}

	prb := &pfRuleBuilder{
		iface:  nx.tunnelIface,
		pfFile: filepath.Join(nx.stateDir, "pf.conf"),
	}

	// Copy the main PacketForwarding configuration file /etc/pf.conf to tmp. Nexodus does
	// not alter the original system file to avoid any issues with OS upgrades etc.
	if err := copyFile(basePFFile, prb.pfFile); err != nil {
		return fmt.Errorf("failed to copy pf.conf: %w", err)
	}

	// Check for com.apple.internet-sharing anchor. UTM (qemu) and possibly others are enabling
	// bridge mode sharing outside of anchor files. This checks if that anchor is loaded and if it
	// is it adds it to the copy of the main pf entry.
	if isAppleSharingEnabled, err := checkAppleInternetSharing(); err == nil && isAppleSharingEnabled {
		if err := appendAppleSharingAnchor(prb.pfFile); err != nil {
			return fmt.Errorf("failed to append Apple Internet Sharing anchor: %w", err)
		}
	}

	// Add io.nexodus anchor entry to the copy of pf.conf
	if err := appendNexodusAnchor(prb.pfFile); err != nil {
		return fmt.Errorf("failed to append io.nexodus anchor: %w", err)
	}

	// Explicit drop if rules are defined
	if len(nx.securityGroup.InboundRules) > 0 {
		prb.pfBlockAll("in")
	}

	// Process inbound rules
	for _, rule := range nx.securityGroup.InboundRules {
		if len(rule.IpRanges) == 0 || containsEmptyRange(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "inbound"); err != nil {
				nx.logger.Errorf("pfctl setup error, failed to process inbound rule with 'any': %v", err)
				return fmt.Errorf("pfctl setup error, failed to process inbound rule with 'any': %w", err)
			}
		} else if util.ContainsValidCustomIPv4Ranges(rule.IpRanges) || util.ContainsValidCustomIPv6Ranges(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAddr(rule, "inbound"); err != nil {
				nx.logger.Errorf("pfctl setup error, failed to process inbound rule: %v", err)
				return fmt.Errorf("pfctl setup error, failed to process inbound rule: %w", err)
			}
		} else {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "inbound"); err != nil {
				nx.logger.Errorf("pfctl setup error, failed to process inbound rule with 'any': %v", err)
				return fmt.Errorf("pfctl setup error, failed to process inbound rule with 'any': %w", err)
			}
		}
	}

	// Explicit drop if rules are defined
	if len(nx.securityGroup.OutboundRules) > 0 {
		prb.pfBlockAll("out")
	}

	// Process outbound rules
	for _, rule := range nx.securityGroup.OutboundRules {
		if len(rule.IpRanges) == 0 || containsEmptyRange(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "outbound"); err != nil {
				nx.logger.Errorf("pfctl setup error, failed to process outbound rule with 'any': %v", err)
				return fmt.Errorf("pfctl setup error, failed to process outbound rule with 'any': %w", err)
			}
		} else if util.ContainsValidCustomIPv4Ranges(rule.IpRanges) || util.ContainsValidCustomIPv6Ranges(rule.IpRanges) {
			if err := prb.pfPermitProtoPortAddr(rule, "outbound"); err != nil {
				nx.logger.Errorf("pfctl setup error, failed to process outbound rule: %v", err)
				return fmt.Errorf("pfctl setup error, failed to process outbound rule: %w", err)
			}
		} else {
			if err := prb.pfPermitProtoPortAnyAddr(rule, "outbound"); err != nil {
				nx.logger.Errorf("pfctl setup error, failed to process outbound rule with 'any': %v", err)
				return fmt.Errorf("pfctl setup error, failed to process outbound rule with 'any': %w", err)
			}
		}
	}

	// Open the anchor file to write pf rules
	f, err := os.OpenFile(pfAnchorFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open pf anchor file: %w", err)
	}
	defer f.Close()

	// If debugging is enabled print the rules in a readable block
	if nx.logger.Level().Enabled(zapcore.InfoLevel) {
		fmt.Println("Generated pfctl Rules:")
		fmt.Println(prb.sb.String())
	}

	_, err = f.WriteString(prb.sb.String())
	if err != nil {
		return fmt.Errorf("failed to write to build the PF rules: %w", err)
	}

	// Overwrite the contents of /etc/pf.anchors/io.nexodus
	if err := os.WriteFile(pfAnchorFile, []byte(prb.sb.String()+"\n"), 0600); err != nil {
		return fmt.Errorf("failed to write to /etc/pf.anchors/io.nexodus: %w", err)
	}

	// Load the pf rules from anchor file
	if _, err := policyCmd(nx.logger, []string{"-f", prb.pfFile}); err != nil {
		return fmt.Errorf("failed to load pf rules: %w", err)
	}

	return nil
}

func (prb *pfRuleBuilder) pfPermitProtoPortAddr(rule public.ModelsSecurityRule, direction string) error {
	var portOption string
	var directionToken string

	if direction == "inbound" {
		directionToken = "pass in"
	} else if direction == "outbound" {
		directionToken = "pass out"
	}

	if rule.FromPort == 0 && rule.ToPort == 0 {
		portOption = ""
	} else {
		portOption = fmt.Sprintf("port %d:%d", rule.FromPort, rule.ToPort)
	}

	ipRangesStr := strings.Join(rule.IpRanges, ", ")
	// Ensure there are spaces around any dashes
	ipRangesStr = strings.ReplaceAll(ipRangesStr, "-", " - ")

	ipDirection := "to any"
	if direction == "inbound" {
		ipDirection = fmt.Sprintf("from { %s } to any", ipRangesStr)
	} else if direction == "outbound" {
		ipDirection = fmt.Sprintf("to { %s }", ipRangesStr)
	}

	protocol := rule.IpProtocol
	inetType := "inet"
	if protocol == "ipv6" || protocol == "icmp6" || protocol == "icmpv6" {
		inetType = "inet6"
	}

	switch protocol {
	case "ipv4", "ipv6":
		if portOption != "" {
			prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto tcp %s %s\n", directionToken, prb.iface, inetType, ipDirection, portOption))
			prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto udp %s %s\n", directionToken, prb.iface, inetType, ipDirection, portOption))
		} else {
			prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s %s %s\n", directionToken, prb.iface, inetType, ipDirection, portOption))
		}
	case "tcp", "udp":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto %s %s %s\n", directionToken, prb.iface, inetType, protocol, ipDirection, portOption))
	case "icmp4", "icmpv4":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s inet proto icmp %s\n", directionToken, prb.iface, ipDirection))
	case "icmp6", "icmpv6":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s inet6 proto icmp6 %s\n", directionToken, prb.iface, ipDirection))
	case "icmp":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s inet proto icmp to any\n", directionToken, prb.iface))
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s inet6 proto icmp6 to any\n", directionToken, prb.iface))
	default:
		return fmt.Errorf("no match for permit proto port/port/address rule: %v", rule)
	}

	return nil
}

func (prb *pfRuleBuilder) pfPermitProtoPortAnyAddr(rule public.ModelsSecurityRule, direction string) error {
	var portOption string
	var directionToken string

	if direction == "inbound" {
		directionToken = "pass in"
	} else if direction == "outbound" {
		directionToken = "pass out"
	}

	if rule.FromPort == 0 && rule.ToPort == 0 {
		portOption = ""
	} else {
		portOption = fmt.Sprintf("port %d:%d", rule.FromPort, rule.ToPort)
	}

	ipDirection := "to any"
	if direction == "inbound" {
		ipDirection = "from any to any"
	} else if direction == "outbound" {
		ipDirection = "to any"
	}

	protocol := rule.IpProtocol
	inetType := "inet"
	if protocol == "ipv6" || protocol == "icmp6" || protocol == "icmpv6" {
		inetType = "inet6"
	}

	switch protocol {
	case "tcp", "udp":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto %s %s %s\n", directionToken, prb.iface, inetType, protocol, ipDirection, portOption))
	case "ipv4", "ipv6":
		if portOption != "" {
			prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto tcp %s %s\n", directionToken, prb.iface, inetType, ipDirection, portOption))
			prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto udp %s %s\n", directionToken, prb.iface, inetType, ipDirection, portOption))
		} else {
			prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s %s %s\n", directionToken, "utun8", inetType, ipDirection, portOption))
		}
	case "icmp4", "icmpv4":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto icmp %s\n", directionToken, prb.iface, inetType, ipDirection))
	case "icmp6", "icmpv6":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s %s proto icmp6 %s\n", directionToken, prb.iface, inetType, ipDirection))
	case "icmp":
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s inet proto icmp %s\n", directionToken, prb.iface, ipDirection))
		prb.sb.WriteString(fmt.Sprintf("%s quick on %s inet6 proto icmp6 %s\n", directionToken, prb.iface, ipDirection))
	default:
		return fmt.Errorf("no policy PF match for permit proto port any address rule: %v", rule)
	}

	return nil
}

// copyFile Copy file from src to dst
func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0600)
}

// checkAppleInternetSharing Check if com.apple.internet-sharing anchor is loaded.
// This is used by various macOS hypervisors and not loaded via the main PF config.
func checkAppleInternetSharing() (bool, error) {
	output, err := exec.Command("pfctl", "-sA").CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(output), appleSharingAnchor), nil
}

// appendToFileAfterLastOccurrence Appends the given lines to the file after the last
// occurrence of the specified keyword. PF anchors have to be loaded in specific order
func appendToFileAfterLastOccurrence(filename string, keyword string, linesToAppend string) error {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(fileContent), "\n")

	insertIndex := -1

	// Find the last occurrence of the keyword
	for i, line := range lines {
		if strings.Contains(line, keyword) {
			insertIndex = i + 1
		}
	}

	if insertIndex == -1 {
		return fmt.Errorf("keyword '%s' not found in file", keyword)
	}

	// Insert the linesToAppend after the last occurrence of the keyword
	lines = append(lines[:insertIndex], append([]string{linesToAppend}, lines[insertIndex:]...)...)

	// Write the updated lines back to the file
	return os.WriteFile(filename, []byte(strings.Join(lines, "\n")), 0600)
}

// appendAppleSharingAnchor Append Apple Internet Sharing anchor to tempPFFile
func appendAppleSharingAnchor(tempPFFile string) error {
	anchorLines := fmt.Sprintf("nat-anchor \"%s/*\"\nrdr-anchor \"%s/*\"\n", appleSharingAnchor, appleSharingAnchor)
	return appendToFileAfterLastOccurrence(tempPFFile, "rdr-anchor", anchorLines)
}

// appendNexodusAnchor Append io.nexodus anchor to tempPFFile
func appendNexodusAnchor(tempPFFile string) error {
	anchorLines := "anchor \"io.nexodus\"\nload anchor \"io.nexodus\" from \"/etc/pf.anchors/io.nexodus\"\n"
	return appendToFile(tempPFFile, anchorLines)
}

// appendToFile Append lines to file
func appendToFile(filename, lines string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.WriteString(lines); err != nil {
		return err
	}
	return nil
}

// pfBlockAll adds an implicit drop once an explicit allow is added
func (prb *pfRuleBuilder) pfBlockAll(direction string) {
	prb.sb.WriteString(fmt.Sprintf("block %s on %s all\n", direction, prb.iface))
}

// containsEmptyString checks if the slice contains an empty string
func containsEmptyRange(ranges []string) bool {
	for _, ipRange := range ranges {
		if ipRange == "" {
			return true
		}
	}
	return false
}

// checkAndEnablePfctl checks if pf is running and if it isn't, enable it.
func checkAndEnablePfctl(logger *zap.SugaredLogger) error {
	cmd := exec.Command("pfctl", "-s", "info")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run pfctl info: %w", err)
	}

	if !strings.Contains(string(output), "Status: Enabled") {
		if _, err := policyCmd(logger, []string{"-e"}); err != nil {
			return fmt.Errorf("failed to enable pfctl: %w", err)
		}
	}

	return nil
}

// writeEmptyRuleSet write the PF rules to the anchorFile
func writeEmptyRuleSet(filePath string) error {
	content := []byte("pass quick on utun8 all")
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		return fmt.Errorf("failed to write to anchor file: %w", err)
	}

	return nil
}

// policyCmd is used to execute pfctl commands
func policyCmd(logger *zap.SugaredLogger, cmdArgs []string) (string, error) {
	pfctl := exec.Command("pfctl", cmdArgs...)

	output, err := pfctl.CombinedOutput()
	if err != nil {
		logger.Errorw("pfctl command failed", "error", err, "output", string(output))
		return "", fmt.Errorf("pfctl command: pfctl %q failed: %w", strings.Join(cmdArgs, " "), err)
	}
	logger.Debugf("pfctl command: pfctl %s", strings.Join(cmdArgs, " "))

	return string(output), nil
}

// networkRouterSetup network router currently unsupported on darwin
func (nx *Nexodus) networkRouterSetup() error {
	return nil
}

// policyTableDrop for Darwin build purposes
func (nx *Nexodus) policyTableDrop(table string) error {
	return nil
}
