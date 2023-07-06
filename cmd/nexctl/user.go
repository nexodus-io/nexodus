package main

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v2"
	"log"
)

func userTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "USER ID", Field: "Id"})
	fields = append(fields, TableField{Header: "USER NAME", Field: "UserName"})
	return fields
}
func listUsers(cCtx *cli.Context, c *client.APIClient) error {
	users, _, err := c.UsersApi.ListUsers(context.Background()).Execute()
	if err != nil {
		log.Fatal(err)
	}
	showOutput(cCtx, userTableFields(), users)
	return nil
}

func deleteUser(cCtx *cli.Context, c *client.APIClient, encodeOut, userID string) error {
	res, _, err := c.UsersApi.DeleteUser(context.Background(), userID).Execute()
	if err != nil {
		log.Fatalf("user delete failed: %v\n", err)
	}

	showOutput(cCtx, userTableFields(), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println("\nsuccessfully deleted")
	}

	return nil
}

func deleteUserFromOrg(cCtx *cli.Context, c *client.APIClient, encodeOut, userID, orgID string) error {
	res, _, err := c.UsersApi.DeleteUserFromOrganization(context.Background(), userID, orgID).Execute()
	if err != nil {
		log.Fatalf("user removal failed: %v\n", err)
	}

	showOutput(cCtx, userTableFields(), res)
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully removed user %s from organization %s\n", userID, orgID)
	}
	return nil
}

func getCurrent(cCtx *cli.Context, c *client.APIClient) error {
	user, _, err := c.UsersApi.GetUser(context.Background(), "me").Execute()
	if err != nil {
		log.Fatal(err)
	}
	showOutput(cCtx, userTableFields(), user)
	return nil
}
