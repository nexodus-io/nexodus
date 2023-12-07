package main

import (
	"context"
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
				Action: func(command *cli.Context) error {
					return listUsers(command.Context, command)
				},
			},
			{
				Name:  "get-current",
				Usage: "Get current user",
				Action: func(command *cli.Context) error {
					return getCurrent(command.Context, command)
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
				Action: func(command *cli.Context) error {
					userID, err := getUUID(command, "user-id")
					if err != nil {
						return err
					}
					return deleteUser(command.Context, command, userID)
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
				Action: func(command *cli.Context) error {
					userID, err := getUUID(command, "user-id")
					if err != nil {
						return err
					}
					orgID, err := getUUID(command, "organization-id")
					if err != nil {
						return err
					}
					return deleteUserFromOrg(command.Context, command, userID, orgID)
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
func listUsers(ctx context.Context, command *cli.Context) error {
	c := createClient(ctx, command)
	res := apiResponse(c.UsersApi.
		ListUsers(ctx).
		Execute())
	show(command, userTableFields(), res)
	return nil
}

func deleteUser(ctx context.Context, command *cli.Context, userID string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.UsersApi.
		DeleteUser(ctx, userID).
		Execute())
	show(command, userTableFields(), res)
	showSuccessfully(command, "deleted")
	return nil
}

func deleteUserFromOrg(ctx context.Context, command *cli.Context, userID, orgID string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.UsersApi.
		DeleteUserFromOrganization(ctx, userID, orgID).
		Execute())
	show(command, userTableFields(), res)
	showSuccessfully(command, fmt.Sprintf("removed user %s from organization %s\n", userID, orgID))
	return nil
}

func getCurrent(ctx context.Context, command *cli.Context) error {
	c := createClient(ctx, command)
	res := apiResponse(c.UsersApi.
		GetUser(ctx, "me").
		Execute())
	show(command, userTableFields(), res)
	return nil
}
