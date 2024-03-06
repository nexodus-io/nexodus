package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v3"
)

func createSecurityGroupCommand() *cli.Command {
	return &cli.Command{
		Name:  "security-group",
		Usage: "commands relating to security groups",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all security groups",
				Action: func(ctx context.Context, command *cli.Command) error {
					return listSecurityGroups(ctx, command)
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
				Action: func(ctx context.Context, command *cli.Command) error {
					encodeOut := command.String("output")
					sgID, err := getUUID(command, "security-group-id")
					if err != nil {
						return err
					}

					return deleteSecurityGroup(ctx, command, encodeOut, sgID)
				},
			},
			{
				Name:  "create",
				Usage: "create a security group",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "vpc-id",
						Required: false,
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
				Action: func(ctx context.Context, command *cli.Command) error {
					description := command.String("description")
					vpcId, err := getUUID(command, "vpc-id")
					if err != nil {
						return err
					}

					inboundRulesStr := command.String("inbound-rules")
					outboundRulesStr := command.String("outbound-rules")

					var inboundRules, outboundRules []client.ModelsSecurityRule
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

					return createSecurityGroup(ctx, command, description, vpcId, inboundRules, outboundRules)
				},
			},
			{
				Name:  "update",
				Usage: "update a security group",
				Flags: []cli.Flag{
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
				Action: func(ctx context.Context, command *cli.Command) error {

					update := client.ModelsUpdateSecurityGroup{}

					id, err := getUUID(command, "security-group-id")
					if err != nil {
						return err
					}

					if command.IsSet("description") {
						update.Description = client.PtrString(command.String("description"))
					}
					if command.IsSet("inbound-rules") {
						rules, err := jsonStringToSecurityRules(command.String("inbound-rules"))
						if err != nil {
							return fmt.Errorf("failed to convert inbound rules string to security rules: %w", err)
						}
						update.InboundRules = rules
					}
					if command.IsSet("outbound-rules") {
						rules, err := jsonStringToSecurityRules(command.String("outbound-rules"))
						if err != nil {
							return fmt.Errorf("failed to convert outbound rules string to security rules: %w", err)
						}
						update.OutboundRules = rules
					}

					err = checkICMPRules(update.InboundRules, update.InboundRules)
					if err != nil {
						return fmt.Errorf("update security group failed: %w", err)
					}

					return updateSecurityGroup(ctx, command, id, update)
				},
			},
		},
	}
}

func securityGroupTableFields(command *cli.Command) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "Id"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
	fields = append(fields, TableField{Header: "INBOUND RULES", Field: "InboundRules"})
	fields = append(fields, TableField{Header: "OUTBOUND RULES", Field: "OutboundRules"})
	return fields
}

// createSecurityGroup creates a new security group.
func createSecurityGroup(ctx context.Context, command *cli.Command, description, vpcId string, inboundRules, outboundRules []client.ModelsSecurityRule) error {
	c := createClient(ctx, command)
	if vpcId == "" {
		vpcId = getDefaultVpcId(ctx, c)
	}
	err := checkICMPRules(inboundRules, outboundRules)
	if err != nil {
		return fmt.Errorf("invalid rules: %w", err)
	}
	res := apiResponse(c.SecurityGroupApi.CreateSecurityGroup(ctx).SecurityGroup(client.ModelsAddSecurityGroup{
		Description:   client.PtrString(description),
		VpcId:         client.PtrString(vpcId),
		InboundRules:  inboundRules,
		OutboundRules: outboundRules,
	}).Execute())
	show(command, securityGroupTableFields(command), res)
	return nil
}

// updateSecurityGroup updates an existing security group.
func updateSecurityGroup(ctx context.Context, command *cli.Command, secGroupID string, update client.ModelsUpdateSecurityGroup) error {
	c := createClient(ctx, command)
	res := apiResponse(c.SecurityGroupApi.
		UpdateSecurityGroup(ctx, secGroupID).
		Update(update).
		Execute())
	show(command, securityGroupTableFields(command), res)
	showSuccessfully(command, "updated")
	return nil
}

// listSecurityGroups lists all security groups.
func listSecurityGroups(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	res := apiResponse(c.SecurityGroupApi.
		ListSecurityGroups(ctx).
		Execute())
	show(command, securityGroupTableFields(command), res)
	return nil
}

// deleteSecurityGroup deletes an existing security group.
func deleteSecurityGroup(ctx context.Context, command *cli.Command, encodeOut, secGroupID string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.SecurityGroupApi.
		DeleteSecurityGroup(ctx, secGroupID).
		Execute())
	show(command, securityGroupTableFields(command), res)
	showSuccessfully(command, "deleted")
	return nil
}

func jsonStringToSecurityRules(jsonString string) ([]client.ModelsSecurityRule, error) {
	var rules []client.ModelsSecurityRule
	err := json.Unmarshal([]byte(jsonString), &rules)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal security rules: %w", err)
	}
	return rules, nil
}

// checkICMPRules prevents the user from defining ICMP rules with ports set to anything but 0.
func checkICMPRules(inboundRules []client.ModelsSecurityRule, outboundRules []client.ModelsSecurityRule) error {
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
func checkICMPRule(rule client.ModelsSecurityRule) error {
	if rule.GetIpProtocol() == "icmp" && (rule.GetFromPort() != 0 || rule.GetToPort() != 0) {
		return fmt.Errorf("error: ICMP rule should have FromPort and ToPort set to 0 or left undefined")
	}
	return nil
}
