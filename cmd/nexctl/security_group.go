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

func createSecurityGroupCommand() *cli.Command {
	return &cli.Command{
		Name:  "security-group",
		Usage: "commands relating to security groups",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all security groups",
				Action: func(cCtx *cli.Context) error {
					return listSecurityGroups(cCtx, mustCreateAPIClient(cCtx))
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a security group",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "security-group-id",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					encodeOut := cCtx.String("output")
					sgID := cCtx.String("security-group-id")
					return deleteSecurityGroup(cCtx, mustCreateAPIClient(cCtx), encodeOut, sgID)
				},
			},
			{
				Name:  "create",
				Usage: "create a security group",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "organization-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "inbound-rules",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "outbound-rules",
						Required: false,
					},
				},
				Action: func(cCtx *cli.Context) error {
					name := cCtx.String("name")
					description := cCtx.String("description")
					orgID := cCtx.String("organization-id")
					inboundRulesStr := cCtx.String("inbound-rules")
					outboundRulesStr := cCtx.String("outbound-rules")

					var inboundRules, outboundRules []public.ModelsSecurityRule
					var err error

					if inboundRulesStr != "" {
						inboundRules, err = jsonStringToSecurityRules(inboundRulesStr)
						if err != nil {
							return fmt.Errorf("failed to convert inbound rules string to security rules: %w", err)
						}
					}

					if outboundRulesStr != "" {
						outboundRules, err = jsonStringToSecurityRules(outboundRulesStr)
						if err != nil {
							return fmt.Errorf("failed to convert outbound rules string to security rules: %w", err)
						}
					}

					return createSecurityGroup(cCtx, mustCreateAPIClient(cCtx), name, description, orgID, inboundRules, outboundRules)
				},
			},
			{
				Name:  "update",
				Usage: "update a security group",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "security-group-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "inbound-rules",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "outbound-rules",
						Required: false,
					},
				},
				Action: func(cCtx *cli.Context) error {

					update := public.ModelsUpdateSecurityGroup{}

					id := cCtx.String("security-group-id")
					if cCtx.IsSet("name") {
						update.GroupName = cCtx.String("name")
					}
					if cCtx.IsSet("description") {
						update.GroupDescription = cCtx.String("description")
					}
					if cCtx.IsSet("inbound-rules") {
						rules, err := jsonStringToSecurityRules(cCtx.String("inbound-rules"))
						if err != nil {
							return fmt.Errorf("failed to convert inbound rules string to security rules: %w", err)
						}
						update.InboundRules = rules
					}
					if cCtx.IsSet("outbound-rules") {
						rules, err := jsonStringToSecurityRules(cCtx.String("outbound-rules"))
						if err != nil {
							return fmt.Errorf("failed to convert outbound rules string to security rules: %w", err)
						}
						update.OutboundRules = rules
					}

					err := checkICMPRules(update.InboundRules, update.InboundRules)
					if err != nil {
						return fmt.Errorf("update security group failed: %w", err)
					}

					return updateSecurityGroup(cCtx, mustCreateAPIClient(cCtx), id, update)
				},
			},
		},
	}
}

func securityGroupTableFields(cCtx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "Id"})
	fields = append(fields, TableField{Header: "SECURITY GROUP NAME", Field: "GroupName"})
	fields = append(fields, TableField{Header: "SECURITY GROUP DESCRIPTION", Field: "GroupDescription"})
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "OrganizationId"})
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

	res, httpResp, err := c.SecurityGroupApi.CreateSecurityGroup(context.Background()).SecurityGroup(public.ModelsAddSecurityGroup{
		GroupName:        name,
		GroupDescription: description,
		OrganizationId:   orgID.String(),
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
func updateSecurityGroup(cCtx *cli.Context, c *client.APIClient, secGroupID string, update public.ModelsUpdateSecurityGroup) error {

	res, httpResp, err := c.SecurityGroupApi.UpdateSecurityGroup(context.Background(), secGroupID).Update(update).Execute()
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
func listSecurityGroups(cCtx *cli.Context, c *client.APIClient) error {
	securityGroups, _, err := c.SecurityGroupApi.ListSecurityGroups(context.Background()).Execute()
	if err != nil {
		return fmt.Errorf("list security groups failed: %w", err)
	}

	showOutput(cCtx, securityGroupTableFields(cCtx), securityGroups)
	return nil
}

// deleteSecurityGroup deletes an existing security group.
func deleteSecurityGroup(cCtx *cli.Context, c *client.APIClient, encodeOut, secGroupID string) error {
	res, _, err := c.SecurityGroupApi.DeleteSecurityGroup(context.Background(), secGroupID).Execute()
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
