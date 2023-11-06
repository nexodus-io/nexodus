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

func createRegKeyCommand() *cli.Command {
	return &cli.Command{
		Name:  "reg-key",
		Usage: "Commands relating to registration keys",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List registration keys",
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
				},
				Action: func(cCtx *cli.Context) error {
					return createRegKey(cCtx, mustCreateAPIClient(cCtx), public.ModelsAddRegKey{
						VpcId:       cCtx.String("vpc-id"),
						Description: cCtx.String("description"),
						ExpiresAt:   toExpiration(cCtx.Duration("expiration")),
						SingleUse:   cCtx.Bool("single-use"),
					})
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a registration key",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "id",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					encodeOut := cCtx.String("output")
					id := cCtx.String("id")
					return deleteRegKey(cCtx, mustCreateAPIClient(cCtx), encodeOut, id)
				},
			},
		},
	}
}

func regTokenTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "TOKEN ID", Field: "Id"})
	fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "SINGLE USE", Formatter: func(item interface{}) string {
		if item.(public.ModelsRegKey).DeviceId == "" {
			return "false"
		} else {
			return "true"
		}
	}})
	fields = append(fields, TableField{Header: "EXPIRES AT", Field: "ExpiresAt"})
	fields = append(fields, TableField{Header: "BEARER TOKEN", Field: "BearerToken"})
	return fields
}

func listRegKeys(cCtx *cli.Context, c *client.APIClient) error {
	rows, _, err := c.RegKeyApi.ListRegKeys(cCtx.Context).Execute()
	if err != nil {
		log.Fatal(err)
	}

	showOutput(cCtx, regTokenTableFields(), rows)
	return nil
}

func createRegKey(cCtx *cli.Context, c *client.APIClient, token public.ModelsAddRegKey) error {

	if token.VpcId == "" {
		token.VpcId = getDefaultVpcId(cCtx.Context, c)
	}

	res, _, err := c.RegKeyApi.CreateRegKey(cCtx.Context).RegKey(token).Execute()
	if err != nil {
		log.Fatal(err)
	}
	showOutput(cCtx, regTokenTableFields(), res)
	return nil
}

func getDefaultOrgId(ctx context.Context, c *client.APIClient) string {
	user, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
	if err != nil {
		log.Fatal(err)
	}
	return user.Id
}

func getDefaultVpcId(ctx context.Context, c *client.APIClient) string {
	user, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
	if err != nil {
		log.Fatal(err)
	}
	return user.Id
}

func deleteRegKey(cCtx *cli.Context, c *client.APIClient, encodeOut, id string) error {
	tokenId, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s: %v", id, err)
	}

	res, _, err := c.RegKeyApi.DeleteRegKey(cCtx.Context, tokenId.String()).Execute()
	if err != nil {
		log.Fatalf("Registration token delete failed: %v\n", err)
	}

	showOutput(cCtx, regTokenTableFields(), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println("\nsuccessfully deleted")
	}

	return nil
}
