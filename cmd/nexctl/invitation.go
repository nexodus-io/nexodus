package main

import (
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
)

func createInvitationCommand() *cli.Command {
	return &cli.Command{
		Name:  "invitation",
		Usage: "commands relating to invitations",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List invitations",
				Action: func(ctx *cli.Context) error {
					return listInvitations(ctx)
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
				Action: func(ctx *cli.Context) error {
					organizationId, err := getUUID(ctx, "organization-id")
					if err != nil {
						return err
					}
					userId, err := getUUID(ctx, "user-id")
					if err != nil {
						return err
					}
					return createInvitation(ctx, public.ModelsAddInvitation{
						OrganizationId: organizationId,
						UserId:         userId,
						UserName:       ctx.String("user-name"),
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
				Action: func(ctx *cli.Context) error {
					id, err := getUUID(ctx, "inv-id")
					if err != nil {
						return err
					}
					return deleteInvitation(ctx, id)
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
				Action: func(ctx *cli.Context) error {
					id, err := getUUID(ctx, "inv-id")
					if err != nil {
						return err
					}
					return acceptInvitation(ctx, id)
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

func listInvitations(ctx *cli.Context) error {
	c := createClient(ctx)
	res := apiResponse(c.InvitationApi.
		ListInvitations(ctx.Context).
		Execute())
	show(ctx, invitationsTableFields(), res)
	return nil
}
func acceptInvitation(ctx *cli.Context, id string) error {
	c := createClient(ctx)
	httpResp, err := c.InvitationApi.
		AcceptInvitation(ctx.Context, id).
		Execute()
	_ = apiResponse("", httpResp, err)
	showSuccessfully(ctx, "accepted")
	return nil
}

func deleteInvitation(ctx *cli.Context, id string) error {
	c := createClient(ctx)
	res := apiResponse(c.InvitationApi.
		DeleteInvitation(ctx.Context, id).
		Execute())
	show(ctx, invitationsTableFields(), res)
	showSuccessfully(ctx, "deleted")
	return nil
}

func createInvitation(ctx *cli.Context, invitation public.ModelsAddInvitation) error {
	c := createClient(ctx)
	if invitation.OrganizationId == "" {
		invitation.OrganizationId = getDefaultOrgId(ctx.Context, c)
	}
	if invitation.UserId == "" && invitation.UserName == "" {
		return fmt.Errorf("either the --user-id or --user-name flags are required")
	}
	res := apiResponse(c.InvitationApi.
		CreateInvitation(ctx.Context).
		Invitation(invitation).
		Execute())
	show(ctx, invitationsTableFields(), res)
	return nil
}
