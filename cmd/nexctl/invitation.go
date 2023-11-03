package main

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
	"log"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
)

func createInvitationCommand() *cli.Command {
	return &cli.Command{
		Name:  "invitation",
		Usage: "commands relating to invitations",
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "create an invitation",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "organization-id",
						Required: false,
					},
				},
				Action: func(cCtx *cli.Context) error {
					encodeOut := cCtx.String("output")
					userID := cCtx.String("user-id")
					orgID := cCtx.String("organization-id")
					return createInvitation(mustCreateAPIClient(cCtx), encodeOut, userID, orgID)
				},
			},
			{
				Name:  "delete",
				Usage: "delete an invitation",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "inv-id",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					userID := cCtx.String("inv-id")
					return deleteInvitation(mustCreateAPIClient(cCtx), userID)
				},
			},
			{
				Name:  "accept",
				Usage: "accept an invitation",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "inv-id",
						Required: true,
					},
				},
				Action: func(cCtx *cli.Context) error {
					userID := cCtx.String("inv-id")
					return acceptInvitation(mustCreateAPIClient(cCtx), userID)
				},
			},
		},
	}
}

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
	if orgID == "" {
		orgID = getDefaultOwnedOrgId(context.Background(), c)
	}
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
