package main

import (
	"fmt"
	"log"
	"os"

	"github.com/redhat-et/apex/internal/client"
	"github.com/urfave/cli/v2"
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
							res, err := c.ListZones()
							if err != nil {
								log.Fatal(err)
							}
							fmt.Printf("%+v", res)
							return nil
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
							res, err := c.CreateZone(
								cCtx.String("name"),
								cCtx.String("description"),
								cCtx.String("cidr"),
								cCtx.Bool("hub-zone"),
							)
							if err != nil {
								log.Fatal(err)
							}
							fmt.Printf("%+v", res)
							return nil
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
