package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
				Value: "https://api.try.nexodus.local",
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
				Name:  "nexd",
				Usage: "Commands for interacting with the local instance of nexd",
				Subcommands: []*cli.Command{
					{
						Name:   "version",
						Usage:  "Display the nexd version",
						Action: cmdLocalVersion,
					},
					{
						Name:   "status",
						Usage:  "Display the nexd status",
						Action: cmdLocalStatus,
					},
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
							c, err := client.NewClient(cCtx.Context,
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
							&cli.BoolFlag{
								Name:  "hub-organization",
								Value: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
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
							organizationName := cCtx.String("name")
							organizationDescrip := cCtx.String("description")
							organizationCIDR := cCtx.String("cidr")
							organizationHub := cCtx.Bool("hub-organization")
							return createOrganization(c, encodeOut, organizationName, organizationDescrip, organizationCIDR, organizationHub)
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
							c, err := client.NewClient(cCtx.Context,
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
							c, err := client.NewClient(cCtx.Context,
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
							c, err := client.NewClient(cCtx.Context,
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
							c, err := client.NewClient(cCtx.Context,
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
							return listUsers(c, encodeOut)
						},
					},
					{
						Name:  "get-current",
						Usage: "Get current user",
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
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
							c, err := client.NewClient(cCtx.Context,
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
							c, err := client.NewClient(cCtx.Context,
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
							orgID := cCtx.String("organization-id")
							return deleteUserFromOrg(c, encodeOut, userID, orgID)
						},
					},
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
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
