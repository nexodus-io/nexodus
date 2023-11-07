package main

import (
	"context"
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
				Name:  "list",
				Usage: "List invitations",
				Action: func(cCtx *cli.Context) error {
					return listInvitations(cCtx, mustCreateAPIClient(cCtx))
				},
			},
			{
				Name:  "create",
				Usage: "create an invitation",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "user-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "user-name",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "organization-id",
						Required: false,
					},
				},
				Action: func(cCtx *cli.Context) error {
					return createInvitation(cCtx, mustCreateAPIClient(cCtx), public.ModelsAddInvitation{
						OrganizationId: cCtx.String("organization-id"),
						UserId:         cCtx.String("user-id"),
						UserName:       cCtx.String("user-name"),
					})
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
					return deleteInvitation(cCtx, mustCreateAPIClient(cCtx), userID)
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

func invitationsTableFields() []TableField {
	var fields []TableField
	fields = append(fields, TableField{Header: "INVITATION ID", Field: "Id"})
	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "OrganizationId"})
	fields = append(fields, TableField{Header: "USER ID", Field: "UserId"})
	fields = append(fields, TableField{Header: "EXPIRES AT", Field: "ExpiresAt"})
	return fields
}

func listInvitations(cCtx *cli.Context, c *client.APIClient) error {
	rows := processApiResponse(c.InvitationApi.ListInvitations(cCtx.Context).Execute())

	showOutput(cCtx, invitationsTableFields(), rows)
	return nil
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

func deleteInvitation(cCtx *cli.Context, c *client.APIClient, id string) error {
	invID, err := uuid.Parse(id)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", id, err)
	}
	res := processApiResponse(c.InvitationApi.DeleteInvitation(context.Background(), invID.String()).Execute())
	showOutput(cCtx, invitationsTableFields(), res)
	return nil
}

func createInvitation(cCtx *cli.Context, c *client.APIClient, invitation public.ModelsAddInvitation) error {

	if invitation.OrganizationId == "" {
		invitation.OrganizationId = getDefaultOrgId(context.Background(), c)
	}
	_, err := uuid.Parse(invitation.OrganizationId)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", invitation.OrganizationId, err)
	}

	if invitation.UserId == "" && invitation.UserName == "" {
		log.Fatalf("either --user-id or --user-name must be specified")
	}

	res := processApiResponse(c.InvitationApi.CreateInvitation(context.Background()).Invitation(invitation).Execute())
	showOutput(cCtx, invitationsTableFields(), res)
	return nil
}
