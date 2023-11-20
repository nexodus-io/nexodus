package main

import (
	"strings"
	"time"

	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
)

const LocalTimeFormat = "2006-01-02 15:04:05 MST"

func createDeviceCommand() *cli.Command {
	return &cli.Command{
		Name:  "device",
		Usage: "Commands relating to devices",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all devices",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "vpc-id",
						Value:    "",
						Required: false,
					},
					&cli.BoolFlag{
						Name:    "full",
						Aliases: []string{"f"},
						Usage:   "display the full set of device details",
						Value:   false,
					},
				},
				Action: func(ctx *cli.Context) error {
					vpcId, err := getUUID(ctx, "vpc-id")
					if err != nil {
						return err
					}
					if vpcId != "" {
						return listVpcDevices(ctx, vpcId)
					}
					return listAllDevices(ctx)
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
				Action: func(ctx *cli.Context) error {
					devID, err := getUUID(ctx, "device-id")
					if err != nil {
						return err
					}
					return deleteDevice(ctx, devID)
				},
			},
			{
				Name:  "update",
				Usage: "Update a device",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "device-id",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "security-group-id",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "hostname",
						Required: false,
					},
				},
				Action: func(ctx *cli.Context) error {

					devID, err := getUUID(ctx, "device-id")
					if err != nil {
						return err
					}

					update := public.ModelsUpdateDevice{}
					if ctx.IsSet("hostname") {
						value := ctx.String("hostname")
						update.Hostname = value
					}
					if ctx.IsSet("security-group-id") {
						value, err := getUUID(ctx, "security-group-id")
						if err != nil {
							return err
						}
						update.SecurityGroupId = value
					}
					return updateDevice(ctx, devID, update)
				},
			},
			{
				Name:        "metadata",
				Usage:       "Commands relating to device metadata",
				Subcommands: deviceMetadataSubcommands,
			},
		},
	}
}
func deviceTableFields(ctx *cli.Context) []TableField {
	var fields []TableField
	full := ctx.Bool("full")

	fields = append(fields, TableField{Header: "DEVICE ID", Field: "Id"})
	fields = append(fields, TableField{Header: "HOSTNAME", Field: "Hostname"})
	fields = append(fields, TableField{Header: "TUNNEL IPS",
		Formatter: func(item interface{}) string {
			dev := item.(public.ModelsDevice)
			ips := []string{}
			for _, ip := range dev.Ipv4TunnelIps {
				ips = append(ips, ip.Address)
			}
			for _, ip := range dev.Ipv6TunnelIps {
				ips = append(ips, ip.Address)
			}
			return strings.Join(ips, ", ")
		},
	})

	fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
	fields = append(fields, TableField{Header: "RELAY", Field: "Relay"})
	if full {
		fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
		fields = append(fields, TableField{Header: "LOCAL IP", Formatter: func(item interface{}) string {
			dev := item.(public.ModelsDevice)
			for _, endpoint := range dev.Endpoints {
				if endpoint.Source == "local" {
					return endpoint.Address
				}
			}
			return ""
		}})
		fields = append(fields, TableField{Header: "ADVERTISED CIDR", Formatter: func(item interface{}) string {
			dev := item.(public.ModelsDevice)
			return strings.Join(dev.AllowedIps, ", ")
		}})
		fields = append(fields, TableField{Header: "REFLEXIVE IPv4", Formatter: func(item interface{}) string {
			dev := item.(public.ModelsDevice)
			var reflexiveIp4 []string
			for _, endpoint := range dev.Endpoints {
				if endpoint.Source != "local" {
					reflexiveIp4 = append(reflexiveIp4, endpoint.Address)
				}
			}
			return strings.Join(reflexiveIp4, ", ")
		}})
		fields = append(fields, TableField{Header: "LOCAL IPv4", Formatter: func(item interface{}) string {
			dev := item.(public.ModelsDevice)
			var localIp4 []string
			for _, endpoint := range dev.Endpoints {
				if endpoint.Source == "local" {
					localIp4 = append(localIp4, endpoint.Address)
				}
			}
			return strings.Join(localIp4, ", ")
		}})
		fields = append(fields, TableField{Header: "OS", Field: "Os"})
		fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "SecurityGroupId"})
		fields = append(fields, TableField{Header: "ONLINE", Field: "Online"})
		fields = append(fields, TableField{Header: "ONLINE SINCE", Formatter: func(item interface{}) string {
			d := item.(public.ModelsDevice)
			if !d.Online {
				return ""
			}
			parsedTime, err := time.Parse(time.RFC3339, d.OnlineAt)
			if err != nil {
				return d.OnlineAt
			}
			localTime := parsedTime.Local()
			return localTime.Format(LocalTimeFormat)
		}})
	}
	return fields
}

func listAllDevices(ctx *cli.Context) error {
	c := createClient(ctx)
	res := apiResponse(c.DevicesApi.
		ListDevices(ctx.Context).
		Execute())
	show(ctx, deviceTableFields(ctx), res)
	return nil
}

func listVpcDevices(ctx *cli.Context, vpcId string) error {
	c := createClient(ctx)
	response := apiResponse(c.VPCApi.
		ListDevicesInVPC(ctx.Context, vpcId).
		Execute())
	show(ctx, deviceTableFields(ctx), response)
	return nil
}

func deleteDevice(ctx *cli.Context, devID string) error {
	c := createClient(ctx)
	res := apiResponse(c.DevicesApi.
		DeleteDevice(ctx.Context, devID).
		Execute())
	show(ctx, deviceTableFields(ctx), res)
	showSuccessfully(ctx, "deleted")
	return nil
}

func updateDevice(ctx *cli.Context, devID string, update public.ModelsUpdateDevice) error {
	c := createClient(ctx)
	res := apiResponse(c.DevicesApi.
		UpdateDevice(ctx.Context, devID).
		Update(update).
		Execute())
	show(ctx, deviceTableFields(ctx), res)
	showSuccessfully(ctx, "updated")
	return nil
}
