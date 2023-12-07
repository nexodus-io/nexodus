package main

import (
	"context"
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
				Action: func(command *cli.Context) error {
					return listVPCs(command.Context, command)
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
				Action: func(command *cli.Context) error {
					return createVPC(command.Context, command, public.ModelsAddVPC{
						Ipv4Cidr:       command.String("ipv4-cidr"),
						Ipv6Cidr:       command.String("ipv6-cidr"),
						Description:    command.String("description"),
						OrganizationId: command.String("organization-id"),
						PrivateCidr:    !(command.String("ipv4-cidr") == "" && command.String("ipv6-cidr") == ""),
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
				Action: func(command *cli.Context) error {
					id, err := getUUID(command, "vpc-id")
					if err != nil {
						return err
					}

					update := public.ModelsUpdateVPC{
						Description: command.String("description"),
					}
					return updateVPC(command.Context, command, id, update)
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
				Action: func(command *cli.Context) error {
					vpcID, err := getUUID(command, "vpc-id")
					if err != nil {
						return err
					}
					return deleteVPC(command.Context, command, vpcID)
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

func updateVPC(ctx context.Context, command *cli.Context, idStr string, update public.ModelsUpdateVPC) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		Fatalf("failed to parse a valid UUID from %s %v", idStr, err)
	}

	c := createClient(ctx, command)
	res := apiResponse(c.VPCApi.
		UpdateVPC(ctx, id.String()).
		Update(update).
		Execute())

	show(command, invitationsTableFields(), res)
	showSuccessfully(command, "updated")
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
func listVPCs(ctx context.Context, command *cli.Context) error {
	c := createClient(ctx, command)
	res := apiResponse(c.VPCApi.
		ListVPCs(ctx).
		Execute())
	show(command, vpcTableFields(), res)
	return nil
}

func createVPC(ctx context.Context, command *cli.Context, resource public.ModelsAddVPC) error {
	c := createClient(ctx, command)
	if resource.OrganizationId == "" {
		resource.OrganizationId = getDefaultOrgId(ctx, c)
	}
	res := apiResponse(c.VPCApi.
		CreateVPC(ctx).
		VPC(resource).
		Execute())
	show(command, vpcTableFields(), res)
	return nil
}

func deleteVPC(ctx context.Context, command *cli.Context, id string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.VPCApi.
		DeleteVPC(ctx, id).
		Execute())
	show(command, vpcTableFields(), res)
	showSuccessfully(command, "deleted")
	return nil
}
