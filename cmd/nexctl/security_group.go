package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v2"
)

func securityGroupTableFields(cCtx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "Id"})
	fields = append(fields, TableField{Header: "SECURITY GROUP NAME", Field: "GroupName"})
	fields = append(fields, TableField{Header: "SECURITY GROUP DESCRIPTION", Field: "GroupDescription"})
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "OrgId"})
	fields = append(fields, TableField{Header: "SECURITY GROUP RULES INBOUND", Field: "InboundRules"})
	fields = append(fields, TableField{Header: "SECURITY GROUP RULES OUTBOUND", Field: "OutboundRules"})
	return fields
}

// createSecurityGroup creates a new security group.
func createSecurityGroup(cCtx *cli.Context, c *client.APIClient, name, description, organizationID string, inboundRules, outboundRules []public.ModelsSecurityRule) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}

	err = checkICMPRules(inboundRules, outboundRules)
	if err != nil {
		return fmt.Errorf("create security group failed: %w", err)
	}

	res, httpResp, err := c.SecurityGroupApi.CreateSecurityGroup(context.Background(), orgID.String()).SecurityGroup(public.ModelsAddSecurityGroup{
		GroupName:        name,
		GroupDescription: description,
		OrgId:            orgID.String(),
		InboundRules:     inboundRules,
		OutboundRules:    outboundRules,
	}).Execute()
	if err != nil {
		// Decode the body for better logging of a rule with a field that doesn't conform to sanity checks
		if httpResp != nil && httpResp.StatusCode == http.StatusUnprocessableEntity {
			var validationErr public.ModelsValidationError
			decodeErr := json.NewDecoder(httpResp.Body).Decode(&validationErr)
			if decodeErr != nil {
				return fmt.Errorf("create security group failed and error decoding: %w", decodeErr)
			}
			return fmt.Errorf("create security group validation failed: %s - %s", validationErr.Field, validationErr.Error)
		}
		return fmt.Errorf("create security group failed: %w", err)
	}

	showOutput(cCtx, securityGroupTableFields(cCtx), res)
	return nil
}

// updateSecurityGroup updates an existing security group.
func updateSecurityGroup(cCtx *cli.Context, c *client.APIClient, secGroupID, organizationID, name, description string, inboundRules, outboundRules []public.ModelsSecurityRule) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}

	err = checkICMPRules(inboundRules, outboundRules)
	if err != nil {
		return fmt.Errorf("create security group failed: %w", err)
	}

	res, httpResp, err := c.SecurityGroupApi.UpdateSecurityGroup(context.Background(), orgID.String(), secGroupID).Update(public.ModelsUpdateSecurityGroup{
		GroupName:        name,
		GroupDescription: description,
		InboundRules:     inboundRules,
		OutboundRules:    outboundRules,
	}).Execute()
	if err != nil {
		// Decode the body for better logging of a rule with a field that doesn't conform to sanity checks
		if httpResp != nil && httpResp.StatusCode == http.StatusUnprocessableEntity {
			var validationErr public.ModelsValidationError
			decodeErr := json.NewDecoder(httpResp.Body).Decode(&validationErr)
			if decodeErr != nil {
				return fmt.Errorf("update security group failed and error decoding: %w", decodeErr)
			}
			return fmt.Errorf("update security group validation failed: %s - %s", validationErr.Field, validationErr.Error)
		}
		return fmt.Errorf("update security group failed: %w", err)
	}

	showOutput(cCtx, securityGroupTableFields(cCtx), res)
	return nil
}

// listSecurityGroups lists all security groups.
func listSecurityGroups(cCtx *cli.Context, c *client.APIClient, encodeOut string, organizationID string) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}
	securityGroups, _, err := c.SecurityGroupApi.ListSecurityGroups(context.Background(), orgID.String()).Execute()
	if err != nil {
		return fmt.Errorf("list security groups failed: %w", err)
	}

	showOutput(cCtx, securityGroupTableFields(cCtx), securityGroups)
	return nil
}

// deleteSecurityGroup deletes an existing security group.
func deleteSecurityGroup(cCtx *cli.Context, c *client.APIClient, encodeOut, secGroupID, organizationID string) error {
	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return fmt.Errorf("failed to parse a valid UUID from %s %w", organizationID, err)
	}
	res, _, err := c.SecurityGroupApi.DeleteSecurityGroup(context.Background(), orgID.String(), secGroupID).Execute()
	if err != nil {
		return fmt.Errorf("security group delete failed: %w", err)
	}

	showOutput(cCtx, securityGroupTableFields(cCtx), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println("\nsuccessfully deleted")
	}

	return nil
}

func jsonStringToSecurityRules(jsonString string) ([]public.ModelsSecurityRule, error) {
	var rules []public.ModelsSecurityRule
	err := json.Unmarshal([]byte(jsonString), &rules)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal security rules: %w", err)
	}
	return rules, nil
}

// checkICMPRules prevents the user from defining ICMP rules with ports set to anything but 0.
func checkICMPRules(inboundRules []public.ModelsSecurityRule, outboundRules []public.ModelsSecurityRule) error {
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
func checkICMPRule(rule public.ModelsSecurityRule) error {
	if rule.IpProtocol == "icmp" && (rule.FromPort != 0 || rule.ToPort != 0) {
		return fmt.Errorf("error: ICMP rule should have FromPort and ToPort set to 0 or left undefined")
	}
	return nil
}
