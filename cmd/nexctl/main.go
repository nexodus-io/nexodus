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
	app := &cli.App{
		Name: "apexctl",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Value:   false,
				Usage:   "enable debug logging",
				EnvVars: []string{"APEX_DEBUG"},
			},
			&cli.StringFlag{
				Name:  "host",
				Value: "https://api.nexodus.local",
				Usage: "api server",
			},
			&cli.StringFlag{
				Name:  "username",
				Usage: "username",
			},
			&cli.StringFlag{
				Name:  "password",
				Usage: "password",
			},
			&cli.StringFlag{
				Name:     "output",
				Value:    encodeColumn,
				Required: false,
				Usage:    "output format: json, json-raw, no-header, column (default columns)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Get the version of apexctl",
				Action: func(cCtx *cli.Context) error {
					fmt.Printf("version: %s\n", Version)
					return nil
				},
			},
			{
				Name:  "apexd",
				Usage: "commands for interacting with the local instance of apexd",
				Subcommands: []*cli.Command{
					{
						Name:   "version",
						Usage:  "apexd version",
						Action: cmdLocalVersion,
					},
					{
						Name:   "status",
						Usage:  "apexd status",
						Action: cmdLocalStatus,
					},
				},
			},
			{
				Name:  "organization",
				Usage: "commands relating to organizations",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list organizations",
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
						Usage: "create a organizations",
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
						Usage: "delete a organization",
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
				Usage: "commands relating to devices",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list all devices",
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
						Usage: "delete a device",
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
				Usage: "commands relating to users",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list all users",
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
						Usage: "get current user",
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
						Usage: "delete a user",
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
