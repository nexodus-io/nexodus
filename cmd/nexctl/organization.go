package main

import (
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
)

func createOrganizationCommand() *cli.Command {
	return &cli.Command{
		Name:  "organization",
		Usage: "Commands relating to organizations",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List organizations",
				Action: func(ctx *cli.Context) error {
					return listOrganizations(ctx)
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
				Action: func(ctx *cli.Context) error {
					name := ctx.String("name")
					description := ctx.String("description")
					return createOrganization(ctx, name, description)
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
				Action: func(ctx *cli.Context) error {
					organizationID, err := getUUID(ctx, "organization-id")
					if err != nil {
						return err
					}

					return deleteOrganization(ctx, organizationID)
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
	return fields
}
func listOrganizations(ctx *cli.Context) error {
	c := createClient(ctx)
	res := apiResponse(c.OrganizationsApi.
		ListOrganizations(ctx.Context).
		Execute())
	show(ctx, orgTableFields(), res)
	return nil
}

func createOrganization(ctx *cli.Context, name, description string) error {
	c := createClient(ctx)
	res := apiResponse(c.OrganizationsApi.
		CreateOrganization(ctx.Context).
		Organization(public.ModelsAddOrganization{
			Name:        name,
			Description: description,
		}).Execute())
	show(ctx, orgTableFields(), res)
	return nil
}

/*
func moveUserToOrganization(c *client.APIClient, encodeOut, username, OrganizationID string) error {
	OrganizationUUID, err := uuid.Parse(OrganizationID)
	if err != nil {
		Fatalf("failed to parse a valid UUID from %s %v", OrganizationID, err)
	}

	res, err := c.MoveCurrentUserToOrganization(OrganizationUUID)
	if err != nil {
		Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("%s successfully moved into Organization %s\n", username, OrganizationID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		Fatalf("failed to print output: %v", err)
	}

	return nil
}
*/

func deleteOrganization(ctx *cli.Context, id string) error {
	c := createClient(ctx)
	res := apiResponse(c.OrganizationsApi.
		DeleteOrganization(ctx.Context, id).
		Execute())
	show(ctx, orgTableFields(), res)
	showSuccessfully(ctx, "deleted")
	return nil
}
