package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v3"
)

func createVpcCommand() *cli.Command {
	return &cli.Command{
		Name:  "vpc",
		Usage: "Commands relating to vpcs",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List vpcs",
				Action: func(ctx context.Context, command *cli.Command) error {
					return listVPCs(ctx, command)
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
				Action: func(ctx context.Context, command *cli.Command) error {
					return createVPC(ctx, command, client.ModelsAddVPC{
						Ipv4Cidr:       client.PtrOptionalString(command.String("ipv4-cidr")),
						Ipv6Cidr:       client.PtrOptionalString(command.String("ipv6-cidr")),
						Description:    client.PtrOptionalString(command.String("description")),
						OrganizationId: client.PtrOptionalString(command.String("organization-id")),
						PrivateCidr:    client.PtrBool(!(command.String("ipv4-cidr") == "" && command.String("ipv6-cidr") == "")),
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
				Action: func(ctx context.Context, command *cli.Command) error {
					id, err := getUUID(command, "vpc-id")
					if err != nil {
						return err
					}

					update := client.ModelsUpdateVPC{
						Description: client.PtrString(command.String("description")),
					}
					return updateVPC(ctx, command, id, update)
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
				Action: func(ctx context.Context, command *cli.Command) error {
					vpcID, err := getUUID(command, "vpc-id")
					if err != nil {
						return err
					}
					return deleteVPC(ctx, command, vpcID)
				},
			},
			{
				Name:     "metadata",
				Usage:    "Commands relating to device metadata across the vpc",
				Commands: vpcMetadataSubcommands,
			},
		},
	}
}

func updateVPC(ctx context.Context, command *cli.Command, idStr string, update client.ModelsUpdateVPC) error {
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
func listVPCs(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	res := apiResponse(c.VPCApi.
		ListVPCs(ctx).
		Execute())
	show(command, vpcTableFields(), res)
	return nil
}

func createVPC(ctx context.Context, command *cli.Command, resource client.ModelsAddVPC) error {
	c := createClient(ctx, command)
	if resource.GetOrganizationId() == "" {
		resource.OrganizationId = client.PtrString(getDefaultOrgId(ctx, c))
	}
	res := apiResponse(c.VPCApi.
		CreateVPC(ctx).
		VPC(resource).
		Execute())
	show(command, vpcTableFields(), res)
	return nil
}

func deleteVPC(ctx context.Context, command *cli.Command, id string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.VPCApi.
		DeleteVPC(ctx, id).
		Execute())
	show(command, vpcTableFields(), res)
	showSuccessfully(command, "deleted")
	return nil
}
