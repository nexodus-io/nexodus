package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/redhat-et/apex/internal/client"
	"github.com/urfave/cli/v2"
)

const (
	encodeJsonRaw    = "json-raw"
	encodeJsonPretty = "json"
	encodeNoHeader   = "no-header"
	encodeColumn     = "column"
)

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
				Value: "https://api.apex.local",
				Usage: "api server",
			},
			&cli.StringFlag{
				Name:     "username",
				Required: true,
				Usage:    "username",
			},
			&cli.StringFlag{
				Name:     "password",
				Required: true,
				Usage:    "password",
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
				Name:  "zone",
				Usage: "commands relating to zones",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list zones",
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							return listZones(c, encodeOut)
						},
					},
					{
						Name:  "create",
						Usage: "create a zones",
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
								Name:  "hub-zone",
								Value: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							zoneName := cCtx.String("name")
							zoneDescrip := cCtx.String("description")
							zoneCIDR := cCtx.String("cidr")
							zoneHub := cCtx.Bool("hub-zone")
							return createZone(c, encodeOut, zoneName, zoneDescrip, zoneCIDR, zoneHub)
						},
					},
					{
						Name:  "move-user",
						Usage: "move the current user to a zone",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "zone-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							zoneID := cCtx.String("zone-id")
							username := cCtx.String("username")
							return moveUserToZone(c, encodeOut, username, zoneID)
						},
					},
					{
						Name:  "delete",
						Usage: "delete a zone",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "zone-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							zoneID := cCtx.String("zone-id")
							return deleteZone(c, encodeOut, zoneID)
						},
					},
				},
			},
			{
				Name:  "peer",
				Usage: "commands relating to zones",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "list all peers in a zone",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "zone-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							zoneID := cCtx.String("zone-id")
							return listPeersInZone(c, encodeOut, zoneID)
						},
					},
					{
						Name:  "list-all",
						Usage: "list all peers",
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							return listAllPeers(c, encodeOut)
						},
					},
					{
						Name:  "delete",
						Usage: "delete a peer from all zones",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "peer-id",
								Required: true,
							},
						},
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
							peerID := cCtx.String("peer-id")
							return deletePeer(c, encodeOut, peerID)
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
						Action: func(cCtx *cli.Context) error {
							c, err := client.NewClient(cCtx.Context,
								cCtx.String("host"),
								client.WithPasswordGrant(
									cCtx.String("username"),
									cCtx.String("password"),
								),
							)
							if err != nil {
								log.Fatal(err)
							}
							encodeOut := cCtx.String("output")
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
								cCtx.String("host"),
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
								cCtx.String("host"),
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
								cCtx.String("host"),
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
