package main

import (
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
)

func createRegKeyCommand() *cli.Command {
	return &cli.Command{
		Name:  "reg-key",
		Usage: "Commands relating to registration keys",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List registration keys",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "full",
						Aliases: []string{"f"},
						Usage:   "display the full set of registration key details",
						Value:   false,
					},
				},
				Action: func(ctx *cli.Context) error {
					return listRegKeys(ctx)
				},
			},
			{
				Name:  "create",
				Usage: "Create a registration key",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "vpc-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "security-group-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
					&cli.BoolFlag{
						Name:     "single-use",
						Required: false,
					},
					&cli.DurationFlag{
						Name:     "expiration",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "settings",
						Required: false,
					},
				},
				Action: func(ctx *cli.Context) error {
					settings := map[string]interface{}{}
					if ctx.String("settings") != "" {
						err := json.Unmarshal([]byte(ctx.String("settings")), &settings)
						if err != nil {
							return fmt.Errorf("invalid --settings flag value: %w", err)
						}
					} else {
						settings = nil
					}

					return createRegKey(ctx, public.ModelsAddRegKey{
						VpcId:           ctx.String("vpc-id"),
						Description:     ctx.String("description"),
						ExpiresAt:       getExpiration(ctx, "expiration"),
						SingleUse:       ctx.Bool("single-use"),
						SecurityGroupId: ctx.String("security-group-id"),
						Settings:        settings,
					})
				},
			},
			{
				Name:  "update",
				Usage: "Update a registration key",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "reg-key-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "security-group-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: false,
					},
					&cli.DurationFlag{
						Name:     "expiration",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "settings",
						Required: false,
					},
				},
				Action: func(ctx *cli.Context) error {
					settings := map[string]interface{}{}
					if ctx.String("settings") != "" {
						err := json.Unmarshal([]byte(ctx.String("settings")), &settings)
						if err != nil {
							return fmt.Errorf("invalid --settings flag value: %w", err)
						}
					} else {
						settings = nil
					}

					return updateRegKey(ctx, ctx.String("reg-key-id"), public.ModelsUpdateRegKey{
						Description:     ctx.String("description"),
						ExpiresAt:       getExpiration(ctx, "expiration"),
						SecurityGroupId: ctx.String("security-group-id"),
						Settings:        settings,
					})
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a registration key",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "reg-key-id",
						Required: true,
					},
				},
				Action: func(ctx *cli.Context) error {
					id, err := getUUID(ctx, "reg-key-id")
					if err != nil {
						return err
					}
					return deleteRegKey(ctx, id)
				},
			},
		},
	}
}

func regTokenTableFields(ctx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "TOKEN ID", Field: "Id"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "CLI FLAGS", Formatter: func(item interface{}) string {
		record := item.(public.ModelsRegKey)
		return fmt.Sprintf("--reg-key %s#%s", ctx.String("service-url"), record.BearerToken)
	}})
	if ctx.Bool("full") {
		fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
		fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "SecurityGroupId"})
		fields = append(fields, TableField{Header: "SINGLE USE", Formatter: func(item interface{}) string {
			if item.(public.ModelsRegKey).DeviceId == "" {
				return "false"
			} else {
				return "true"
			}
		}})
		fields = append(fields, TableField{Header: "EXPIRES AT", Field: "ExpiresAt"})
		// fields = append(fields, TableField{Header: "BEARER TOKEN", Field: "BearerToken"})
		fields = append(fields, TableField{Header: "SETTINGS", Field: "Settings"})
	}
	return fields
}

func listRegKeys(ctx *cli.Context) error {
	c := createClient(ctx)
	rows := apiResponse(c.RegKeyApi.
		ListRegKeys(ctx.Context).
		Execute())
	show(ctx, regTokenTableFields(ctx), rows)
	return nil
}

func createRegKey(ctx *cli.Context, token public.ModelsAddRegKey) error {
	c := createClient(ctx)
	if token.VpcId == "" {
		token.VpcId = getDefaultVpcId(ctx.Context, c)
	}
	res := apiResponse(c.RegKeyApi.
		CreateRegKey(ctx.Context).
		RegKey(token).
		Execute())
	show(ctx, regTokenTableFields(ctx), res)
	return nil
}

func updateRegKey(ctx *cli.Context, id string, update public.ModelsUpdateRegKey) error {
	c := createClient(ctx)
	res := apiResponse(c.RegKeyApi.
		UpdateRegKey(ctx.Context, id).
		Update(update).
		Execute())
	show(ctx, regTokenTableFields(ctx), res)
	showSuccessfully(ctx, "updated")
	return nil
}

func deleteRegKey(ctx *cli.Context, id string) error {
	c := createClient(ctx)
	res := apiResponse(c.RegKeyApi.
		DeleteRegKey(ctx.Context, id).
		Execute())
	show(ctx, regTokenTableFields(ctx), res)
	showSuccessfully(ctx, "deleted")
	return nil
}
