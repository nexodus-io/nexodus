package main

import (
	"fmt"
	"log"

	"github.com/redhat-et/apex/internal/client"
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
			fmt.Fprintf(w, fs, "USER ID", "USER NAME", "ZONE ID")
		}

		for _, user := range users {
			fmt.Fprintf(w, fs, user.ID, user.UserName, user.ZoneID)
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
