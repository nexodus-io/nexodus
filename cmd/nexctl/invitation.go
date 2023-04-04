package main

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"log"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
)

func acceptInvitation(c *client.APIClient, id string) error {
	invID, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", id, err)
	}
	if _, err := c.InvitationApi.AcceptInvitation(context.Background(), invID.String()).Execute(); err != nil {
		log.Fatal(err)
	}
	return nil
}

func deleteInvitation(c *client.APIClient, id string) error {
	invID, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", id, err)
	}
	if _, _, err := c.InvitationApi.DeleteInvitation(context.Background(), invID.String()).Execute(); err != nil {
		log.Fatal(err)
	}
	return nil
}

func createInvitation(c *client.APIClient, encodeOut, userID string, orgID string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", orgID, err)
	}
	res, _, err := c.InvitationApi.CreateInvitation(context.Background()).Invitation(public.ModelsAddInvitation{
		UserId:         userID,
		OrganizationId: orgUUID.String(),
	}).Execute()
	if err != nil {
		log.Fatalf("create invitation failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully created invitation %s\n", res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
