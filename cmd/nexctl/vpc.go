package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v2"
	"log"
)

func createVpcCommand() *cli.Command {
	return &cli.Command{
		Name:  "vpc",
		Usage: "Commands relating to vpcs",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List vpcs",
				Action: func(cCtx *cli.Context) error {
					return listVPCs(cCtx, mustCreateAPIClient(cCtx))
				},
			},
			{
				Name:  "create",
				Usage: "Create a vpcs",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "organization-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "cidr",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "cidr-v6",
						Required: false,
					},
				},
				Action: func(cCtx *cli.Context) error {
					return createVPC(cCtx, mustCreateAPIClient(cCtx), public.ModelsAddVPC{
						Cidr:           cCtx.String("cidr"),
						CidrV6:         cCtx.String("cidr-v6"),
						Description:    cCtx.String("description"),
						OrganizationId: cCtx.String("organization-id"),
						PrivateCidr:    !(cCtx.String("cidr") == "" && cCtx.String("cidr-v6") == ""),
					})
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a vpc",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "vpc-id",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					encodeOut := cCtx.String("output")
					vpcID := cCtx.String("vpc-id")
					return deleteVPC(cCtx, mustCreateAPIClient(cCtx), encodeOut, vpcID)
				},
			},
			{
				Name:        "metadata",
				Usage:       "Commands relating to device metadata across the vpc",
				Subcommands: vpcMetadataSubcommands,
			},
		},
	}
}

func vpcTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "Id"})
	fields = append(fields, TableField{Header: "NAME", Field: "Name"})
	fields = append(fields, TableField{Header: "IPV4 CIDR", Field: "Cidr"})
	fields = append(fields, TableField{Header: "IPV6 CIDR", Field: "CidrV6"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "SecurityGroupId"})
	return fields
}
func listVPCs(cCtx *cli.Context, c *client.APIClient) error {
	vpcs, _, err := c.VPCApi.ListVPCs(context.Background()).Execute()
	if err != nil {
		log.Fatal(err)
	}

	showOutput(cCtx, vpcTableFields(), vpcs)
	return nil
}

func createVPC(cCtx *cli.Context, c *client.APIClient, resource public.ModelsAddVPC) error {
	res, _, err := c.VPCApi.CreateVPC(context.Background()).VPC(resource).Execute()
	if err != nil {
		log.Fatal(err)
	}
	showOutput(cCtx, vpcTableFields(), res)
	return nil
}

func deleteVPC(cCtx *cli.Context, c *client.APIClient, encodeOut, VPCID string) error {
	id, err := uuid.Parse(VPCID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", id, err)
	}

	res, _, err := c.VPCApi.DeleteVPC(context.Background(), id.String()).Execute()
	if err != nil {
		log.Fatalf("VPC delete failed: %v\n", err)
	}

	showOutput(cCtx, vpcTableFields(), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println("\nsuccessfully deleted")
	}

	return nil
}
