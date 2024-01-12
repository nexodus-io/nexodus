package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v3"
	"log"
)

func createSiteCommand() *cli.Command {
	return &cli.Command{
		Name:   "site",
		Hidden: true,
		Usage:  "Commands relating to sites",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all sites",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "vpc-id",
						Value:    "",
						Required: false,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					orgID := command.String("vpc-id")
					if orgID != "" {
						id, err := uuid.Parse(orgID)
						if err != nil {
							log.Fatal(err)
						}
						return listVpcSites(ctx, command, id)
					}
					return listAllSites(ctx, command)
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a site",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "site-id",
						Required: true,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					devID := command.String("site-id")
					return deleteSite(ctx, command, devID)
				},
			},
			{
				Name:  "update",
				Usage: "Update a site",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "site-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "hostname",
						Required: false,
					},
				},
				Action: func(ctx context.Context, command *cli.Command) error {
					devID := command.String("site-id")
					update := public.ModelsUpdateSite{}
					if command.IsSet("hostname") {
						update.Hostname = command.String("hostname")
					}
					return updateSite(ctx, command, devID, update)
				},
			},
		},
	}
}
func siteTableFields(command *cli.Command) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "SITE ID", Field: "Id"})
	fields = append(fields, TableField{Header: "HOSTNAME", Field: "Hostname"})
	fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
	fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
	fields = append(fields, TableField{Header: "OS", Field: "Os"})
	return fields
}

func listAllSites(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	sites := apiResponse(c.SitesApi.ListSites(context.Background()).Execute())
	show(command, siteTableFields(command), sites)
	return nil
}

func listVpcSites(ctx context.Context, command *cli.Command, vpcId uuid.UUID) error {
	c := createClient(ctx, command)
	sites := apiResponse(c.VPCApi.ListSitesInVPC(context.Background(), vpcId.String()).Execute())
	show(command, siteTableFields(command), sites)
	return nil
}

func deleteSite(ctx context.Context, command *cli.Command, devID string) error {
	devUUID, err := uuid.Parse(devID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", devUUID, err)
	}
	c := createClient(ctx, command)
	res := apiResponse(c.SitesApi.DeleteSite(context.Background(), devUUID.String()).Execute())
	show(command, orgTableFields(), res)
	showSuccessfully(command, "deleted")
	return nil
}

func updateSite(ctx context.Context, command *cli.Command, devID string, update public.ModelsUpdateSite) error {
	devUUID, err := uuid.Parse(devID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", devUUID, err)
	}

	c := createClient(ctx, command)
	res := apiResponse(c.SitesApi.
		UpdateSite(context.Background(), devUUID.String()).
		Update(update).
		Execute())
	show(command, orgTableFields(), res)
	showSuccessfully(command, "updated")
	return nil
}
