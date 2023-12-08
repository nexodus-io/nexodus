package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v3"
)

func createRegKeyCommand() *cli.Command {
	return &cli.Command{
		Name:  "reg-key",
		Usage: "Commands relating to registration keys",
		Commands: []*cli.Command{
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
				Action: func(ctx context.Context, command *cli.Command) error {
					return listRegKeys(ctx, command)
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
				Action: func(ctx context.Context, command *cli.Command) error {
					settings := map[string]interface{}{}
					if command.String("settings") != "" {
						err := json.Unmarshal([]byte(command.String("settings")), &settings)
						if err != nil {
							return fmt.Errorf("invalid --settings flag value: %w", err)
						}
					} else {
						settings = nil
					}

					return createRegKey(ctx, command, public.ModelsAddRegKey{
						VpcId:           command.String("vpc-id"),
						Description:     command.String("description"),
						ExpiresAt:       getExpiration(command, "expiration"),
						SingleUse:       command.Bool("single-use"),
						SecurityGroupId: command.String("security-group-id"),
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
				Action: func(ctx context.Context, command *cli.Command) error {
					settings := map[string]interface{}{}
					if command.String("settings") != "" {
						err := json.Unmarshal([]byte(command.String("settings")), &settings)
						if err != nil {
							return fmt.Errorf("invalid --settings flag value: %w", err)
						}
					} else {
						settings = nil
					}

					return updateRegKey(ctx, command, command.String("reg-key-id"), public.ModelsUpdateRegKey{
						Description:     command.String("description"),
						ExpiresAt:       getExpiration(command, "expiration"),
						SecurityGroupId: command.String("security-group-id"),
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
				Action: func(ctx context.Context, command *cli.Command) error {
					id, err := getUUID(command, "reg-key-id")
					if err != nil {
						return err
					}
					return deleteRegKey(ctx, command, id)
				},
			},
		},
	}
}

func regTokenTableFields(command *cli.Command) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "TOKEN ID", Field: "Id"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "CLI FLAGS", Formatter: func(item interface{}) string {
		record := item.(public.ModelsRegKey)
		return fmt.Sprintf("--reg-key %s#%s", command.String("service-url"), record.BearerToken)
	}})
	if command.Bool("full") {
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

func listRegKeys(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	rows := apiResponse(c.RegKeyApi.
		ListRegKeys(ctx).
		Execute())
	show(command, regTokenTableFields(command), rows)
	return nil
}

func createRegKey(ctx context.Context, command *cli.Command, token public.ModelsAddRegKey) error {
	c := createClient(ctx, command)
	if token.VpcId == "" {
		token.VpcId = getDefaultVpcId(ctx, c)
	}
	res := apiResponse(c.RegKeyApi.
		CreateRegKey(ctx).
		RegKey(token).
		Execute())
	show(command, regTokenTableFields(command), res)
	return nil
}

func updateRegKey(ctx context.Context, command *cli.Command, id string, update public.ModelsUpdateRegKey) error {
	c := createClient(ctx, command)
	res := apiResponse(c.RegKeyApi.
		UpdateRegKey(ctx, id).
		Update(update).
		Execute())
	show(command, regTokenTableFields(command), res)
	showSuccessfully(command, "updated")
	return nil
}

func deleteRegKey(ctx context.Context, command *cli.Command, id string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.RegKeyApi.
		DeleteRegKey(ctx, id).
		Execute())
	show(command, regTokenTableFields(command), res)
	showSuccessfully(command, "deleted")
	return nil
}
