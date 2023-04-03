package main

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/client"
	"log"
)

func listUsers(c *client.Client, encodeOut string) error {
	users, err := c.ListUsers()
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
			fmt.Fprintf(w, fs, user.ID, user.UserName)
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

func deleteUser(c *client.Client, encodeOut, userID string) error {
	res, err := c.DeleteUser(userID)
	if err != nil {
		log.Fatalf("user delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted user %s\n", res.ID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func deleteUserFromOrg(c *client.Client, encodeOut, userID, orgID string) error {
	res, err := c.DeleteUserFromOrganization(userID, orgID)
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

func getCurrent(c *client.Client, encodeOut string) error {
	user, err := c.GetCurrentUser()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "USER ID", "USER NAME")
		}

		fmt.Fprintf(w, fs, user.ID, user.UserName)

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, user)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
