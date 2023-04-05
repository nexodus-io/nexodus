package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
	"github.com/urfave/cli/v2"
)

const (
	encodeJsonRaw    = "json-raw"
	encodeJsonPretty = "json"
	encodeNoHeader   = "no-header"
	encodeColumn     = "column"
)

// This variable is set using ldflags at build time. See Makefile for details.
var Version = "dev"

var additionalPlatformCommands []*cli.Command = nil

func main() {
	// Override usage to capitalize "Show"
	cli.HelpFlag.(*cli.BoolFlag).Usage = "Show help"
	app := &cli.App{
		Name: "nexctl",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				Usage:   "Enable debug logging",
				EnvVars: []string{"NEXCTL_DEBUG"},
			},
			&cli.StringFlag{
				Name:  "host",
				Value: "https://api.try.nexodus.127.0.0.1.nip.io",
				Usage: "Api server URL",
			},
			&cli.StringFlag{
				Name:  "username",
				Usage: "Username",
			},
			&cli.StringFlag{
				Name:  "password",
				Usage: "Password",
			},
			&cli.StringFlag{
				Name:     "output",
				Value:    encodeColumn,
				Required: false,
				Usage:    "Output format: json, json-raw, no-header, column (default columns)",
			},
			&cli.BoolFlag{
				Name:     "insecure-skip-tls-verify",
				Value:    false,
				Usage:    "If true, server certificates will not be checked for validity. This will make your HTTPS connections insecure",
				Required: false,
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Get the version of nexctl",
				Action: func(cCtx *cli.Context) error {
					fmt.Printf("version: %s\n", Version)
					return nil
				},
			},
			{
				Name:  "organization",
				Usage: "Commands relating to organizations",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List organizations",
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							return listOrganizations(c, encodeOut)
						},
					},
					{
						Name:  "create",
						Usage: "Create a organizations",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "description",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "cidr",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "cidr-v6",
								Required: true,
							},
							&cli.BoolFlag{
								Name:  "hub-organization",
								Value: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							organizationName := cCtx.String("name")
							organizationDescrip := cCtx.String("description")
							organizationCIDR := cCtx.String("cidr")
							organizationCIDRv6 := cCtx.String("cidr-v6")
							organizationHub := cCtx.Bool("hub-organization")
							return createOrganization(c, encodeOut, organizationName, organizationDescrip, organizationCIDR, organizationCIDRv6, organizationHub)
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a organization",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							organizationID := cCtx.String("organization-id")
							return deleteOrganization(c, encodeOut, organizationID)
						},
					},
				},
			},
			{
				Name:  "device",
				Usage: "Commands relating to devices",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List all devices",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "organization-id",
								Value:    "",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {

							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)

							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							orgID := cCtx.String("organization-id")
							if orgID != "" {
								id, err := uuid.Parse(orgID)
								if err != nil {
									log.Fatal(err)
								}
								return listOrgDevices(c, id, encodeOut)
							}
							return listAllDevices(c, encodeOut)
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a device",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "device-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							devID := cCtx.String("device-id")
							return deleteDevice(c, encodeOut, devID)
						},
					},
				},
			},
			{
				Name:  "user",
				Usage: "Commands relating to users",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "List all users",
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							return listUsers(c, encodeOut)
						},
					},
					{
						Name:  "get-current",
						Usage: "Get current user",
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							return getCurrent(c, encodeOut)
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a user",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "user-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							userID := cCtx.String("user-id")
							return deleteUser(c, encodeOut, userID)
						},
					},
					{
						Name:  "remove-user",
						Usage: "Remove a user from an organization",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "user-id",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "organization-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								createClientOptions(cCtx)...,
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							userID := cCtx.String("user-id")
							orgID := cCtx.String("organization-id")
							return deleteUserFromOrg(c, encodeOut, userID, orgID)
						},
					},
				},
			},
			{
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
								Name:     "org-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							userID := cCtx.String("user-id")
							orgID := cCtx.String("org-id")
							return createInvitation(c, encodeOut, userID, orgID)
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
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							userID := cCtx.String("inv-id")
							return deleteInvitation(c, userID)
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
							c, err := client.NewAPIClient(cCtx.Context,
								cCtx.String("host"), nil,
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							userID := cCtx.String("inv-id")
							return acceptInvitation(c, userID)
						},
					},
				},
			},
		},
	}

	app.Commands = append(app.Commands, additionalPlatformCommands...)
	sort.Slice(app.Commands, func(i, j int) bool {
		return app.Commands[i].Name < app.Commands[j].Name
	})

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func createClientOptions(cCtx *cli.Context) []client.Option {
	options := []client.Option{client.WithPasswordGrant(
		cCtx.String("username"),
		cCtx.String("password"),
	)}
	if cCtx.Bool("insecure-skip-tls-verify") { // #nosec G402
		options = append(options, client.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: true,
		}))
	}
	return options
}

func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 10, 1, 5, ' ', 0)
}

func FormatOutput(format string, result interface{}) error {
	switch format {
	case encodeJsonPretty:
		bytes, err := json.MarshalIndent(result, "", "    ")
		if err != nil {
			log.Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	case encodeJsonRaw:
		bytes, err := json.Marshal(result)
		if err != nil {
			log.Fatalf("failed to encode the ctl output: %v", err)
		}
		fmt.Println(string(bytes))

	default:
		return fmt.Errorf("unknown format option: %s", format)
	}

	return nil
}
