package main

import (
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
)

func createVpcCommand() *cli.Command {
	return &cli.Command{
		Name:  "vpc",
		Usage: "Commands relating to vpcs",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List vpcs",
				Action: func(ctx *cli.Context) error {
					return listVPCs(ctx)
				},
			},
			{
				Name:  "create",
				Usage: "Create a vpcs",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "organization-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "ipv4-cidr",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "ipv6-cidr",
						Required: false,
					},
				},
				Action: func(ctx *cli.Context) error {
					return createVPC(ctx, public.ModelsAddVPC{
						Ipv4Cidr:       ctx.String("ipv4-cidr"),
						Ipv6Cidr:       ctx.String("ipv6-cidr"),
						Description:    ctx.String("description"),
						OrganizationId: ctx.String("organization-id"),
						PrivateCidr:    !(ctx.String("ipv4-cidr") == "" && ctx.String("ipv6-cidr") == ""),
					})
				},
			},
			{
				Name:  "update",
				Usage: "Update a vpc",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "vpc-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
				},
				Action: func(ctx *cli.Context) error {
					id, err := getUUID(ctx, "vpc-id")
					if err != nil {
						return err
					}

					update := public.ModelsUpdateVPC{
						Description: ctx.String("description"),
					}
					return updateVPC(ctx, id, update)
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
				Action: func(ctx *cli.Context) error {
					vpcID, err := getUUID(ctx, "vpc-id")
					if err != nil {
						return err
					}
					return deleteVPC(ctx, vpcID)
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

func updateVPC(ctx *cli.Context, idStr string, update public.ModelsUpdateVPC) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		Fatalf("failed to parse a valid UUID from %s %v", idStr, err)
	}

	c := createClient(ctx)
	res := apiResponse(c.VPCApi.
		UpdateVPC(ctx.Context, id.String()).
		Update(update).
		Execute())

	show(ctx, invitationsTableFields(), res)
	showSuccessfully(ctx, "updated")
	return nil
}

func vpcTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "VPC ID", Field: "Id"})
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "OrganizationId"})
	fields = append(fields, TableField{Header: "IPV4 CIDR", Field: "Ipv4Cidr"})
	fields = append(fields, TableField{Header: "IPV6 CIDR", Field: "Ipv6Cidr"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	return fields
}
func listVPCs(ctx *cli.Context) error {
	c := createClient(ctx)
	res := apiResponse(c.VPCApi.
		ListVPCs(ctx.Context).
		Execute())
	show(ctx, vpcTableFields(), res)
	return nil
}

func createVPC(ctx *cli.Context, resource public.ModelsAddVPC) error {
	c := createClient(ctx)
	if resource.OrganizationId == "" {
		resource.OrganizationId = getDefaultOrgId(ctx.Context, c)
	}
	res := apiResponse(c.VPCApi.
		CreateVPC(ctx.Context).
		VPC(resource).
		Execute())
	show(ctx, vpcTableFields(), res)
	return nil
}

func deleteVPC(ctx *cli.Context, id string) error {
	c := createClient(ctx)
	res := apiResponse(c.VPCApi.
		DeleteVPC(ctx.Context, id).
		Execute())
	show(ctx, vpcTableFields(), res)
	showSuccessfully(ctx, "deleted")
	return nil
}
