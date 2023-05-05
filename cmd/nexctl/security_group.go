package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/nexodus-io/nexodus/internal/models"
)

// createSecurityGroup creates a new security group.
func createSecurityGroup(c *client.APIClient, encodeOut, name, description, organizationID string, inboundRules, outboundRules []models.SecurityRuleJson) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}

	inboundRulesJSON, err := json.Marshal(inboundRules)
	if err != nil {
		return fmt.Errorf("failed to marshal inbound rules: %w", err)
	}

	outboundRulesJSON, err := json.Marshal(outboundRules)
	if err != nil {
		return fmt.Errorf("failed to marshal outbound rules: %w", err)
	}

	err = checkICMPRules(inboundRules, outboundRules)
	if err != nil {
		log.Fatal(err)
	}

	res, _, err := c.SecurityGroupApi.CreateSecurityGroup(context.Background(), orgID.String()).SecurityGroup(public.ModelsAddSecurityGroup{
		GroupName:        name,
		GroupDescription: description,
		OrgId:            orgID.String(),
		InboundRules:     string(inboundRulesJSON),
		OutboundRules:    string(outboundRulesJSON),
	}).Execute()
	if err != nil {
		return fmt.Errorf("create security group failed: %w", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println(res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		return fmt.Errorf("failed to print output: %w", err)
	}

	return nil
}

// updateSecurityGroup updates an existing security group.
func updateSecurityGroup(c *client.APIClient, encodeOut, secGroupID, organizationID, name, description string, inboundRules, outboundRules []models.SecurityRuleJson) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}

	inboundRulesJSON, err := json.Marshal(inboundRules)
	if err != nil {
		return fmt.Errorf("failed to marshal inbound rules: %w", err)
	}

	outboundRulesJSON, err := json.Marshal(outboundRules)
	if err != nil {
		return fmt.Errorf("failed to marshal outbound rules: %w", err)
	}

	err = checkICMPRules(inboundRules, outboundRules)
	if err != nil {
		log.Fatal(err)
	}

	res, _, err := c.SecurityGroupApi.UpdateSecurityGroup(context.Background(), orgID.String(), secGroupID).Update(public.ModelsUpdateSecurityGroup{
		GroupName:        name,
		GroupDescription: description,
		InboundRules:     string(inboundRulesJSON),
		OutboundRules:    string(outboundRulesJSON),
	}).Execute()
	if err != nil {
		return fmt.Errorf("update security group failed: %w", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully updated security group %s\n", res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		return fmt.Errorf("failed to print output: %w", err)
	}

	return nil
}

// listSecurityGroups lists all security groups.
func listSecurityGroups(c *client.APIClient, encodeOut string, organizationID string) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}
	securityGroups, _, err := c.SecurityGroupApi.ListSecurityGroups(context.Background(), orgID.String()).Execute()
	if err != nil {
		return fmt.Errorf("list security groups failed: %w", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%+v\t%+v\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "SECURITY GROUP ID", "SECURITY GROUP NAME", "SECURITY GROUP DESCRIPTION", "ORGANIZATION ID", "SECURITY GROUP RULES INBOUND", "SECURITY GROUP RULES INBOUND")
		}

		for _, sg := range securityGroups {
			inboundRules, err := unmarshalSecurityRules(sg.InboundRules)
			if err != nil {
				return fmt.Errorf("failed to parse security group rules: %w", err)
			}
			outboundRules, err := unmarshalSecurityRules(sg.OutboundRules)
			if err != nil {
				return fmt.Errorf("failed to parse security group rules: %w", err)
			}
			fmt.Fprintf(w, fs, sg.Id, sg.GroupName, sg.GroupDescription, sg.OrgId, inboundRules, outboundRules)
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, securityGroups)
	if err != nil {
		return fmt.Errorf("failed to print output: %w", err)
	}

	return nil
}

// deleteSecurityGroup deletes an existing security group.
func deleteSecurityGroup(c *client.APIClient, encodeOut, secGroupID, organizationID string) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}
	res, _, err := c.SecurityGroupApi.DeleteSecurityGroup(context.Background(), secGroupID, orgID.String()).Execute()
	if err != nil {
		return fmt.Errorf("security group delete failed: %w", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted user %s\n", res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		return fmt.Errorf("failed to print output: %w", err)
	}

	return nil
}

func unmarshalSecurityRules(jsonStr string) ([]models.SecurityRuleJson, error) {
	var rules []models.SecurityRuleJson
	err := json.Unmarshal([]byte(jsonStr), &rules)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func jsonStringToSecurityRules(jsonString string) ([]models.SecurityRuleJson, error) {
	var rules []models.SecurityRuleJson
	err := json.Unmarshal([]byte(jsonString), &rules)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal security rules: %w", err)
	}
	return rules, nil
}

// checkICMPRules prevents the user from defining ICMP rules with ports set to anything but 0.
func checkICMPRules(inboundRules []models.SecurityRuleJson, outboundRules []models.SecurityRuleJson) error {
	for _, rule := range inboundRules {
		err := checkICMPRule(rule)
		if err != nil {
			return err
		}
	}
	for _, rule := range outboundRules {
		err := checkICMPRule(rule)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkICMPRule checks an ICMP rules with ports set to anything but 0.
func checkICMPRule(rule models.SecurityRuleJson) error {
	if rule.IpProtocol == "icmp" && (rule.FromPort != 0 || rule.ToPort != 0) {
		return fmt.Errorf("error: ICMP rule should have FromPort and ToPort set to 0 or left undefined")
	}
	return nil
}
