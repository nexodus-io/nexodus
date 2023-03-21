package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/nexodus-io/nexodus/internal/client"
)

func listUsers(c *client.Client, encodeOut string) error {
	users, err := c.ListUsers()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "USER ID", "USER NAME", "ORGANIZATION ID")
		}

		for _, user := range users {
			var orgs []string
			for _, o := range user.Organizations {
				orgs = append(orgs, o.String())
			}
			fmt.Fprintf(w, fs, user.ID, user.UserName, strings.Join(orgs, ","))
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
		fs := "%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "USER ID", "USER NAME", "ORGANIZATION ID")
		}

		var orgs []string
		for _, o := range user.Organizations {
			orgs = append(orgs, o.String())
		}
		fmt.Fprintf(w, fs, user.ID, user.UserName, strings.Join(orgs, ","))

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, user)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
