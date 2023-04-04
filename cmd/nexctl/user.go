package main

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/client"
	"log"
)

func listUsers(c *client.APIClient, encodeOut string) error {
	users, _, err := c.UsersApi.ListUsers(context.Background()).Execute()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "USER ID", "USER NAME")
		}

		for _, user := range users {
			fmt.Fprintf(w, fs, user.Id, user.UserName)
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, users)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func deleteUser(c *client.APIClient, encodeOut, userID string) error {
	res, _, err := c.UsersApi.DeleteUser(context.Background(), userID).Execute()
	if err != nil {
		log.Fatalf("user delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted user %s\n", res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func deleteUserFromOrg(c *client.APIClient, encodeOut, userID, orgID string) error {
	res, _, err := c.UsersApi.DeleteUserFromOrganization(context.Background(), userID, orgID).Execute()
	if err != nil {
		log.Fatalf("user removal failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully removed user %s from organization %s\n", userID, orgID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func getCurrent(c *client.APIClient, encodeOut string) error {
	user, _, err := c.UsersApi.GetUser(context.Background(), "me").Execute()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "USER ID", "USER NAME")
		}

		fmt.Fprintf(w, fs, user.Id, user.UserName)

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, user)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
