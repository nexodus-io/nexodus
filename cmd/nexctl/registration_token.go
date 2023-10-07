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

func regTokenTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "TOKEN ID", Field: "Id"})
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "OrganizationId"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "SINGLE USE", Formatter: func(item interface{}) string {
		if item.(public.ModelsRegistrationToken).DeviceId == "" {
			return "false"
		} else {
			return "true"
		}
	}})
	fields = append(fields, TableField{Header: "EXPIRATION", Field: "Expiration"})
	fields = append(fields, TableField{Header: "BEARER TOKEN", Formatter: func(item interface{}) string {
		token := item.(public.ModelsRegistrationToken).BearerToken
		return wrapLongLines(token, 40)
	}})
	return fields
}

func wrapLongLines(value string, len int) string {
	res := ""
	for i, r := range value {
		if i > 0 && i%len == 0 {
			res += "\n"
		}
		res += string(r)
	}
	return res
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

	if token.OrganizationId == "" {
		token.OrganizationId = getDefaultOwnedOrgId(cCtx.Context, c)
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
