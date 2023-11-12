package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func createUserSubCommand() *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "Commands relating to users",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all users",
				Action: func(ctx *cli.Context) error {
					return listUsers(ctx)
				},
			},
			{
				Name:  "get-current",
				Usage: "Get current user",
				Action: func(ctx *cli.Context) error {
					return getCurrent(ctx)
				},
			},
			{
				Name:  "delete",
				Usage: "Delete a user",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user-id",
						Required: true,
						Hidden:   true,
					},
				},
				Action: func(ctx *cli.Context) error {
					userID, err := getUUID(ctx, "user-id")
					if err != nil {
						return err
					}
					return deleteUser(ctx, userID)
				},
			},
			{
				Name:  "remove-user",
				Usage: "Remove a user from an organization",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "organization-id",
						Required: true,
					},
				},
				Action: func(ctx *cli.Context) error {
					userID, err := getUUID(ctx, "user-id")
					if err != nil {
						return err
					}
					orgID, err := getUUID(ctx, "organization-id")
					if err != nil {
						return err
					}
					return deleteUserFromOrg(ctx, userID, orgID)
				},
			},
		},
	}
}

func userTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "USER ID", Field: "Id"})
	fields = append(fields, TableField{Header: "USER NAME", Field: "Username"})
	return fields
}
func listUsers(ctx *cli.Context) error {
	c := createClient(ctx)
	res := apiResponse(c.UsersApi.
		ListUsers(ctx.Context).
		Execute())
	show(ctx, userTableFields(), res)
	return nil
}

func deleteUser(ctx *cli.Context, userID string) error {
	c := createClient(ctx)
	res := apiResponse(c.UsersApi.
		DeleteUser(ctx.Context, userID).
		Execute())
	show(ctx, userTableFields(), res)
	showSuccessfully(ctx, "deleted")
	return nil
}

func deleteUserFromOrg(ctx *cli.Context, userID, orgID string) error {
	c := createClient(ctx)
	res := apiResponse(c.UsersApi.
		DeleteUserFromOrganization(ctx.Context, userID, orgID).
		Execute())
	show(ctx, userTableFields(), res)
	showSuccessfully(ctx, fmt.Sprintf("removed user %s from organization %s\n", userID, orgID))
	return nil
}

func getCurrent(ctx *cli.Context) error {
	c := createClient(ctx)
	res := apiResponse(c.UsersApi.
		GetUser(ctx.Context, "me").
		Execute())
	show(ctx, userTableFields(), res)
	return nil
}
