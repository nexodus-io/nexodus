package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v3"
)

func createServiceNetworkCommand() *cli.Command {
	return &cli.Command{
		Name:  "service-network",
		Usage: "Commands relating to service networks",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List service networks",
				Action: func(ctx context.Context, command *cli.Command) error {
					return listServiceNetworks(ctx, command)
				},
			},
			{
				Name:  "create",
				Usage: "Create a service network",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "organization-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					return createServiceNetwork(ctx, command, public.ModelsAddServiceNetwork{
						Description:    command.String("description"),
						OrganizationId: command.String("organization-id"),
					})
				},
			},
			{
				Name:  "update",
				Usage: "Update a service network",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "service-network-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					id, err := getUUID(command, "service-network-id")
					if err != nil {
						return err
					}

					update := public.ModelsUpdateServiceNetwork{
						Description: command.String("description"),
					}
					return updateServiceNetwork(ctx, command, id, update)
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a service network",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "service-network-id",
						Required: true,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					serviceNetworkID, err := getUUID(command, "service-network-id")
					if err != nil {
						return err
					}
					return deleteServiceNetwork(ctx, command, serviceNetworkID)
				},
			},
		},
	}
}

func updateServiceNetwork(ctx context.Context, command *cli.Command, idStr string, update public.ModelsUpdateServiceNetwork) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		Fatalf("failed to parse a valid UUID from %s %v", idStr, err)
	}

	c := createClient(ctx, command)
	res := apiResponse(c.ServiceNetworkApi.
		UpdateServiceNetwork(ctx, id.String()).
		Update(update).
		Execute())

	show(command, invitationsTableFields(), res)
	showSuccessfully(command, "updated")
	return nil
}

func serviceNetworkTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "SERVICE NETWORK ID", Field: "Id"})
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "OrganizationId"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	return fields
}
func listServiceNetworks(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	res := apiResponse(c.ServiceNetworkApi.
		ListServiceNetworks(ctx).
		Execute())
	show(command, serviceNetworkTableFields(), res)
	return nil
}

func createServiceNetwork(ctx context.Context, command *cli.Command, resource public.ModelsAddServiceNetwork) error {
	c := createClient(ctx, command)
	if resource.OrganizationId == "" {
		resource.OrganizationId = getDefaultOrgId(ctx, c)
	}
	res := apiResponse(c.ServiceNetworkApi.
		CreateServiceNetwork(ctx).
		ServiceNetwork(resource).
		Execute())
	show(command, serviceNetworkTableFields(), res)
	return nil
}

func deleteServiceNetwork(ctx context.Context, command *cli.Command, id string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.ServiceNetworkApi.
		DeleteServiceNetwork(ctx, id).
		Execute())
	show(command, serviceNetworkTableFields(), res)
	showSuccessfully(command, "deleted")
	return nil
}
