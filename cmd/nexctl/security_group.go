package main

import (
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
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
				Action: func(ctx *cli.Context) error {
					return listSecurityGroups(ctx)
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
				Action: func(ctx *cli.Context) error {
					encodeOut := ctx.String("output")
					sgID, err := getUUID(ctx, "security-group-id")
					if err != nil {
						return err
					}

					return deleteSecurityGroup(ctx, encodeOut, sgID)
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
				Action: func(ctx *cli.Context) error {
					description := ctx.String("description")
					vpcId, err := getUUID(ctx, "vpc-id")
					if err != nil {
						return err
					}

					inboundRulesStr := ctx.String("inbound-rules")
					outboundRulesStr := ctx.String("outbound-rules")

					var inboundRules, outboundRules []public.ModelsSecurityRule
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

					return createSecurityGroup(ctx, description, vpcId, inboundRules, outboundRules)
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
				Action: func(ctx *cli.Context) error {

					update := public.ModelsUpdateSecurityGroup{}

					id, err := getUUID(ctx, "security-group-id")
					if err != nil {
						return err
					}

					if ctx.IsSet("description") {
						update.Description = ctx.String("description")
					}
					if ctx.IsSet("inbound-rules") {
						rules, err := jsonStringToSecurityRules(ctx.String("inbound-rules"))
						if err != nil {
							return fmt.Errorf("failed to convert inbound rules string to security rules: %w", err)
						}
						update.InboundRules = rules
					}
					if ctx.IsSet("outbound-rules") {
						rules, err := jsonStringToSecurityRules(ctx.String("outbound-rules"))
						if err != nil {
							return fmt.Errorf("failed to convert outbound rules string to security rules: %w", err)
						}
						update.OutboundRules = rules
					}

					err = checkICMPRules(update.InboundRules, update.InboundRules)
					if err != nil {
						return fmt.Errorf("update security group failed: %w", err)
					}

					return updateSecurityGroup(ctx, id, update)
				},
			},
		},
	}
}

func securityGroupTableFields(ctx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "Id"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
	fields = append(fields, TableField{Header: "INBOUND RULES", Field: "InboundRules"})
	fields = append(fields, TableField{Header: "OUTBOUND RULES", Field: "OutboundRules"})
	return fields
}

// createSecurityGroup creates a new security group.
func createSecurityGroup(ctx *cli.Context, description, vpcId string, inboundRules, outboundRules []public.ModelsSecurityRule) error {
	c := createClient(ctx)
	if vpcId == "" {
		vpcId = getDefaultVpcId(ctx.Context, c)
	}
	err := checkICMPRules(inboundRules, outboundRules)
	if err != nil {
		return fmt.Errorf("invalid rules: %w", err)
	}
	res := apiResponse(c.SecurityGroupApi.CreateSecurityGroup(ctx.Context).SecurityGroup(public.ModelsAddSecurityGroup{
		Description:   description,
		VpcId:         vpcId,
		InboundRules:  inboundRules,
		OutboundRules: outboundRules,
	}).Execute())
	show(ctx, securityGroupTableFields(ctx), res)
	return nil
}

// updateSecurityGroup updates an existing security group.
func updateSecurityGroup(ctx *cli.Context, secGroupID string, update public.ModelsUpdateSecurityGroup) error {
	c := createClient(ctx)
	res := apiResponse(c.SecurityGroupApi.
		UpdateSecurityGroup(ctx.Context, secGroupID).
		Update(update).
		Execute())
	show(ctx, securityGroupTableFields(ctx), res)
	showSuccessfully(ctx, "updated")
	return nil
}

// listSecurityGroups lists all security groups.
func listSecurityGroups(ctx *cli.Context) error {
	c := createClient(ctx)
	res := apiResponse(c.SecurityGroupApi.
		ListSecurityGroups(ctx.Context).
		Execute())
	show(ctx, securityGroupTableFields(ctx), res)
	return nil
}

// deleteSecurityGroup deletes an existing security group.
func deleteSecurityGroup(ctx *cli.Context, encodeOut, secGroupID string) error {
	c := createClient(ctx)
	res := apiResponse(c.SecurityGroupApi.
		DeleteSecurityGroup(ctx.Context, secGroupID).
		Execute())
	show(ctx, securityGroupTableFields(ctx), res)
	showSuccessfully(ctx, "deleted")
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
