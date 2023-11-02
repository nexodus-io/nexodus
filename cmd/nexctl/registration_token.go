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

func createRegistrationTokenCommand() *cli.Command {
	return &cli.Command{
		Name:  "registration-token",
		Usage: "Commands relating to registration tokens",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List registration tokens",
				Action: func(cCtx *cli.Context) error {
					return listRegistrationTokens(cCtx, mustCreateAPIClient(cCtx))
				},
			},
			{
				Name:  "create",
				Usage: "Create a registration token",
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
					return createRegistrationToken(cCtx, mustCreateAPIClient(cCtx), public.ModelsAddRegistrationToken{
						VpcId:       cCtx.String("vpc-id"),
						Description: cCtx.String("description"),
						Expiration:  toExpiration(cCtx.Duration("expiration")),
						SingleUse:   cCtx.Bool("single-use"),
					})
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a registration token",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "id",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					encodeOut := cCtx.String("output")
					id := cCtx.String("id")
					return deleteRegistrationToken(cCtx, mustCreateAPIClient(cCtx), encodeOut, id)
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
		if item.(public.ModelsRegistrationToken).DeviceId == "" {
			return "false"
		} else {
			return "true"
		}
	}})
	fields = append(fields, TableField{Header: "EXPIRATION", Field: "Expiration"})
	fields = append(fields, TableField{Header: "BEARER TOKEN", Field: "BearerToken"})
	return fields
}

func listRegistrationTokens(cCtx *cli.Context, c *client.APIClient) error {
	rows, _, err := c.RegistrationTokenApi.ListRegistrationTokens(cCtx.Context).Execute()
	if err != nil {
		log.Fatal(err)
	}

	showOutput(cCtx, regTokenTableFields(), rows)
	return nil
}

func createRegistrationToken(cCtx *cli.Context, c *client.APIClient, token public.ModelsAddRegistrationToken) error {

	if token.VpcId == "" {
		token.VpcId = getDefaultVpcId(cCtx.Context, c)
	}

	res, _, err := c.RegistrationTokenApi.CreateRegistrationToken(cCtx.Context).RegistrationToken(token).Execute()
	if err != nil {
		log.Fatal(err)
	}
	showOutput(cCtx, regTokenTableFields(), res)
	return nil
}

func getDefaultOwnedOrgId(ctx context.Context, c *client.APIClient) string {
	orgIds, err := getOwnedOrgIds(ctx, c)
	if err != nil {
		log.Fatal(err)
	}
	if len(orgIds) == 0 {
		log.Fatal("user does not own any organizations, please use the --organization-id flag to specify the organization")
	}
	if len(orgIds) > 1 {
		log.Fatal("user owns multiple organizations, please use the --organization-id flag to specify the organization")
	}
	return orgIds[0]
}

func getDefaultVpcId(ctx context.Context, c *client.APIClient) string {
	orgIds, err := getVpcIds(ctx, c)
	if err != nil {
		log.Fatal(err)
	}
	if len(orgIds) == 0 {
		log.Fatal("user does not own any vpcs, please use the --vpc-id flag to specify the vpc")
	}
	if len(orgIds) > 1 {
		log.Fatal("user has multiple vpcs, please use the --vpc-id flag to specify the vpc")
	}
	return orgIds[0]
}

func getOwnedOrgIds(ctx context.Context, c *client.APIClient) ([]string, error) {
	result := []string{}
	user, _, err := c.UsersApi.GetUser(ctx, "me").Execute()
	if err != nil {
		return result, err
	}
	orgs, _, err := c.OrganizationsApi.ListOrganizations(ctx).Execute()
	if err != nil {
		return result, err
	}
	for _, org := range orgs {
		if org.OwnerId != user.Id {
			continue
		}
		result = append(result, org.Id)
	}
	return result, nil
}

func getVpcIds(ctx context.Context, c *client.APIClient) ([]string, error) {
	result := []string{}
	vpcs, _, err := c.VPCApi.ListVPCs(ctx).Execute()
	if err != nil {
		return result, err
	}
	for _, vpc := range vpcs {
		result = append(result, vpc.Id)
	}
	return result, nil
}
func deleteRegistrationToken(cCtx *cli.Context, c *client.APIClient, encodeOut, id string) error {
	tokenId, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s: %v", id, err)
	}

	res, _, err := c.RegistrationTokenApi.DeleteRegistrationToken(cCtx.Context, tokenId.String()).Execute()
	if err != nil {
		log.Fatalf("Registration token delete failed: %v\n", err)
	}

	showOutput(cCtx, regTokenTableFields(), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println("\nsuccessfully deleted")
	}

	return nil
}
