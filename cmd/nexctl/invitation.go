package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
)

func acceptInvitation(c *client.Client, id string) error {
	invID, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", id, err)
	}
	if err := c.AcceptInvitation(invID); err != nil {
		log.Fatal(err)
	}
	return nil
}

func deleteInvitation(c *client.Client, id string) error {
	invID, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", id, err)
	}
	if err := c.DeleteInvitation(invID); err != nil {
		log.Fatal(err)
	}
	return nil
}

func createInvitation(c *client.Client, encodeOut, userID string, orgID string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", orgID, err)
	}

	res, err := c.CreateInvitation(userID, orgUUID)
	if err != nil {
		log.Fatalf("create invitation failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully created invitation %s\n", res.ID.String())
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
