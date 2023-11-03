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

func createOrganizationCommand() *cli.Command {
	return &cli.Command{
		Name:  "organization",
		Usage: "Commands relating to organizations",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List organizations",
				Action: func(cCtx *cli.Context) error {
					return listOrganizations(cCtx, mustCreateAPIClient(cCtx))
				},
			},
			{
				Name:  "create",
				Usage: "Create a organizations",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "description",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					organizationName := cCtx.String("name")
					organizationDescrip := cCtx.String("description")
					return createOrganization(cCtx, mustCreateAPIClient(cCtx), organizationName, organizationDescrip)
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a organization",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "organization-id",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					encodeOut := cCtx.String("output")
					organizationID := cCtx.String("organization-id")
					return deleteOrganization(cCtx, mustCreateAPIClient(cCtx), encodeOut, organizationID)
				},
			},
		},
	}
}

func orgTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "Id"})
	fields = append(fields, TableField{Header: "NAME", Field: "Name"})
	fields = append(fields, TableField{Header: "DESCRIPTION", Field: "Description"})
	fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "SecurityGroupId"})
	return fields
}
func listOrganizations(cCtx *cli.Context, c *client.APIClient) error {
	orgs, _, err := c.OrganizationsApi.ListOrganizations(context.Background()).Execute()
	if err != nil {
		log.Fatal(err)
	}

	showOutput(cCtx, orgTableFields(), orgs)
	return nil
}

func createOrganization(cCtx *cli.Context, c *client.APIClient, name, description string) error {
	res, _, err := c.OrganizationsApi.CreateOrganization(context.Background()).Organization(public.ModelsAddOrganization{
		Name:        name,
		Description: description,
	}).Execute()
	if err != nil {
		log.Fatal(err)
	}

	showOutput(cCtx, orgTableFields(), res)
	return nil
}

/*
func moveUserToOrganization(c *client.APIClient, encodeOut, username, OrganizationID string) error {
	OrganizationUUID, err := uuid.Parse(OrganizationID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", OrganizationID, err)
	}

	res, err := c.MoveCurrentUserToOrganization(OrganizationUUID)
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("%s successfully moved into Organization %s\n", username, OrganizationID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
*/

func deleteOrganization(cCtx *cli.Context, c *client.APIClient, encodeOut, OrganizationID string) error {
	OrganizationUUID, err := uuid.Parse(OrganizationID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", OrganizationUUID, err)
	}

	res, _, err := c.OrganizationsApi.DeleteOrganization(context.Background(), OrganizationUUID.String()).Execute()
	if err != nil {
		log.Fatalf("Organization delete failed: %v\n", err)
	}

	showOutput(cCtx, orgTableFields(), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println("\nsuccessfully deleted")
	}

	return nil
}
