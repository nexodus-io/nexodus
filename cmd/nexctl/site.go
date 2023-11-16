package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
	"log"
)

func createSiteCommand() *cli.Command {
	return &cli.Command{
		Name:   "site",
		Hidden: true,
		Usage:  "Commands relating to sites",
		Subcommands: []*cli.Command{
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
				Action: func(ctx *cli.Context) error {
					orgID := ctx.String("vpc-id")
					if orgID != "" {
						id, err := uuid.Parse(orgID)
						if err != nil {
							log.Fatal(err)
						}
						return listVpcSites(ctx, id)
					}
					return listAllSites(ctx)
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
				Action: func(ctx *cli.Context) error {
					devID := ctx.String("site-id")
					return deleteSite(ctx, devID)
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
				Action: func(ctx *cli.Context) error {
					devID := ctx.String("site-id")
					update := public.ModelsUpdateSite{}
					if ctx.IsSet("hostname") {
						update.Hostname = ctx.String("hostname")
					}
					return updateSite(ctx, devID, update)
				},
			},
		},
	}
}
func siteTableFields(ctx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "SITE ID", Field: "Id"})
	fields = append(fields, TableField{Header: "HOSTNAME", Field: "Hostname"})
	fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
	fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
	fields = append(fields, TableField{Header: "OS", Field: "Os"})
	return fields
}

func listAllSites(ctx *cli.Context) error {
	c := mustCreateAPIClient(ctx)
	sites := processApiResponse(c.SitesApi.ListSites(context.Background()).Execute())
	showOutput(ctx, siteTableFields(ctx), sites)
	return nil
}

func listVpcSites(ctx *cli.Context, vpcId uuid.UUID) error {
	c := mustCreateAPIClient(ctx)
	sites := processApiResponse(c.VPCApi.ListSitesInVPC(context.Background(), vpcId.String()).Execute())
	showOutput(ctx, siteTableFields(ctx), sites)
	return nil
}

func deleteSite(ctx *cli.Context, devID string) error {
	devUUID, err := uuid.Parse(devID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", devUUID, err)
	}
	c := mustCreateAPIClient(ctx)
	res := processApiResponse(c.SitesApi.DeleteSite(context.Background(), devUUID.String()).Execute())
	showOutput(ctx, orgTableFields(), res)
	return nil
}

func updateSite(ctx *cli.Context, devID string, update public.ModelsUpdateSite) error {
	devUUID, err := uuid.Parse(devID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", devUUID, err)
	}

	c := mustCreateAPIClient(ctx)
	res := processApiResponse(c.SitesApi.
		UpdateSite(context.Background(), devUUID.String()).
		Update(update).
		Execute())
	showOutput(ctx, orgTableFields(), res)

	return nil
}
