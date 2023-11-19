package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v2"
	"log"
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
				Action: func(cCtx *cli.Context) error {
					return listRegKeys(cCtx, mustCreateAPIClient(cCtx))
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
				Action: func(cCtx *cli.Context) error {

					settings := map[string]interface{}{}
					if cCtx.String("settings") != "" {
						err := json.Unmarshal([]byte(cCtx.String("settings")), &settings)
						if err != nil {
							return fmt.Errorf("invalid --settings flag value: %w", err)
						}
					} else {
						settings = nil
					}

					return createRegKey(cCtx, mustCreateAPIClient(cCtx), public.ModelsAddRegKey{
						VpcId:           cCtx.String("vpc-id"),
						Description:     cCtx.String("description"),
						ExpiresAt:       toExpiration(cCtx.Duration("expiration")),
						SingleUse:       cCtx.Bool("single-use"),
						SecurityGroupId: cCtx.String("security-group-id"),
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
				Action: func(cCtx *cli.Context) error {
					settings := map[string]interface{}{}
					if cCtx.String("settings") != "" {
						err := json.Unmarshal([]byte(cCtx.String("settings")), &settings)
						if err != nil {
							return fmt.Errorf("invalid --settings flag value: %w", err)
						}
					} else {
						settings = nil
					}

					return updateRegKey(cCtx, cCtx.String("reg-key-id"), public.ModelsUpdateRegKey{
						Description:     cCtx.String("description"),
						ExpiresAt:       toExpiration(cCtx.Duration("expiration")),
						SecurityGroupId: cCtx.String("security-group-id"),
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
				Action: func(cCtx *cli.Context) error {
					encodeOut := cCtx.String("output")
					id := cCtx.String("reg-key-id")
					return deleteRegKey(cCtx, mustCreateAPIClient(cCtx), encodeOut, id)
				},
			},
		},
	}
}

func regTokenTableFields(cCtx *cli.Context) []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "TOKEN ID", Field: "Id"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "CLI FLAGS", Formatter: func(item interface{}) string {
		record := item.(public.ModelsRegKey)
		return fmt.Sprintf("--reg-key %s#%s", cCtx.String("service-url"), record.BearerToken)
	}})
	if cCtx.Bool("full") {
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

func listRegKeys(cCtx *cli.Context, c *client.APIClient) error {
	rows := processApiResponse(c.RegKeyApi.ListRegKeys(cCtx.Context).Execute())
	showOutput(cCtx, regTokenTableFields(cCtx), rows)
	return nil
}

func createRegKey(cCtx *cli.Context, c *client.APIClient, token public.ModelsAddRegKey) error {

	if token.VpcId == "" {
		token.VpcId = getDefaultVpcId(cCtx.Context, c)
	}

	res := processApiResponse(c.RegKeyApi.CreateRegKey(cCtx.Context).RegKey(token).Execute())
	showOutput(cCtx, regTokenTableFields(cCtx), res)
	return nil
}

func updateRegKey(cCtx *cli.Context, id string, update public.ModelsUpdateRegKey) error {
	showOutput(cCtx, regTokenTableFields(cCtx), processApiResponse(
		mustCreateAPIClient(cCtx).
			RegKeyApi.UpdateRegKey(cCtx.Context, id).
			Update(update).
			Execute(),
	))
	return nil
}
func getDefaultOrgId(ctx context.Context, c *client.APIClient) string {
	user := processApiResponse(c.UsersApi.GetUser(ctx, "me").Execute())
	return user.Id
}

func getDefaultVpcId(ctx context.Context, c *client.APIClient) string {
	user := processApiResponse(c.UsersApi.GetUser(ctx, "me").Execute())
	return user.Id
}

func deleteRegKey(cCtx *cli.Context, c *client.APIClient, encodeOut, id string) error {
	tokenId, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s: %v", id, err)
	}

	res := processApiResponse(c.RegKeyApi.DeleteRegKey(cCtx.Context, tokenId.String()).Execute())

	showOutput(cCtx, regTokenTableFields(cCtx), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println("\nsuccessfully deleted")
	}

	return nil
}
