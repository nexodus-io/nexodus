package main

import (
	"context"
	"github.com/nexodus-io/nexodus/internal/client"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

const LocalTimeFormat = "2006-01-02 15:04:05 MST"

func createDeviceCommand() *cli.Command {
	return &cli.Command{
		Name:  "device",
		Usage: "Commands relating to devices",
		Commands: []*cli.Command{
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
				Action: func(ctx context.Context, command *cli.Command) error {
					vpcId, err := getUUID(command, "vpc-id")
					if err != nil {
						return err
					}
					if vpcId != "" {
						return listVpcDevices(ctx, command, vpcId)
					}
					return listAllDevices(ctx, command)
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
				Action: func(ctx context.Context, command *cli.Command) error {
					devID, err := getUUID(command, "device-id")
					if err != nil {
						return err
					}
					return deleteDevice(ctx, command, devID)
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
				Action: func(ctx context.Context, command *cli.Command) error {

					devID, err := getUUID(command, "device-id")
					if err != nil {
						return err
					}

					update := client.ModelsUpdateDevice{}
					if command.IsSet("hostname") {
						value := command.String("hostname")
						update.Hostname = client.PtrString(value)
					}
					if command.IsSet("security-group-id") {
						value, err := getUUID(command, "security-group-id")
						if err != nil {
							return err
						}
						update.SecurityGroupId = client.PtrString(value)
					}
					return updateDevice(ctx, command, devID, update)
				},
			},
			{
				Name:     "metadata",
				Usage:    "Commands relating to device metadata",
				Commands: deviceMetadataSubcommands,
			},
		},
	}
}
func deviceTableFields(command *cli.Command) []TableField {
	var fields []TableField
	full := command.Bool("full")

	fields = append(fields, TableField{Header: "DEVICE ID", Field: "Id"})
	fields = append(fields, TableField{Header: "HOSTNAME", Field: "Hostname"})
	fields = append(fields, TableField{Header: "TUNNEL IPS",
		Formatter: func(item interface{}) string {
			dev := item.(client.ModelsDevice)
			ips := []string{}
			for _, ip := range dev.Ipv4TunnelIps {
				ips = append(ips, ip.GetAddress())
			}
			for _, ip := range dev.Ipv6TunnelIps {
				ips = append(ips, ip.GetAddress())
			}
			return strings.Join(ips, ", ")
		},
	})

	fields = append(fields, TableField{Header: "VPC ID", Field: "VpcId"})
	fields = append(fields, TableField{Header: "RELAY", Field: "Relay"})
	if full {
		fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
		fields = append(fields, TableField{Header: "LOCAL IP", Formatter: func(item interface{}) string {
			dev := item.(client.ModelsDevice)
			for _, endpoint := range dev.Endpoints {
				if endpoint.GetSource() == "local" {
					return endpoint.GetAddress()
				}
			}
			return ""
		}})
		fields = append(fields, TableField{Header: "ADVERTISED CIDR", Formatter: func(item interface{}) string {
			dev := item.(client.ModelsDevice)
			return strings.Join(dev.AllowedIps, ", ")
		}})
		fields = append(fields, TableField{Header: "REFLEXIVE IPv4", Formatter: func(item interface{}) string {
			dev := item.(client.ModelsDevice)
			var reflexiveIp4 []string
			for _, endpoint := range dev.Endpoints {
				if endpoint.GetSource() != "local" {
					reflexiveIp4 = append(reflexiveIp4, endpoint.GetAddress())
				}
			}
			return strings.Join(reflexiveIp4, ", ")
		}})
		fields = append(fields, TableField{Header: "LOCAL IPv4", Formatter: func(item interface{}) string {
			dev := item.(client.ModelsDevice)
			var localIp4 []string
			for _, endpoint := range dev.Endpoints {
				if endpoint.GetSource() == "local" {
					localIp4 = append(localIp4, endpoint.GetAddress())
				}
			}
			return strings.Join(localIp4, ", ")
		}})
		fields = append(fields, TableField{Header: "SYMMETRIC NAT", Field: "SymmetricNat"})
		fields = append(fields, TableField{Header: "OS", Field: "Os"})
		fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "SecurityGroupId"})
		fields = append(fields, TableField{Header: "ONLINE", Field: "Online"})
		fields = append(fields, TableField{Header: "ONLINE SINCE", Formatter: func(item interface{}) string {
			d := item.(client.ModelsDevice)
			if !d.GetOnline() {
				return ""
			}
			parsedTime, err := time.Parse(time.RFC3339, d.GetOnlineAt())
			if err != nil {
				return d.GetOnlineAt()
			}
			localTime := parsedTime.Local()
			return localTime.Format(LocalTimeFormat)
		}})
	}
	return fields
}

func listAllDevices(ctx context.Context, command *cli.Command) error {
	c := createClient(ctx, command)
	res := apiResponse(c.DevicesApi.
		ListDevices(ctx).
		Execute())
	show(command, deviceTableFields(command), res)
	return nil
}

func listVpcDevices(ctx context.Context, command *cli.Command, vpcId string) error {
	c := createClient(ctx, command)
	response := apiResponse(c.VPCApi.
		ListDevicesInVPC(ctx, vpcId).
		Execute())
	show(command, deviceTableFields(command), response)
	return nil
}

func deleteDevice(ctx context.Context, command *cli.Command, devID string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.DevicesApi.
		DeleteDevice(ctx, devID).
		Execute())
	show(command, deviceTableFields(command), res)
	showSuccessfully(command, "deleted")
	return nil
}

func updateDevice(ctx context.Context, command *cli.Command, devID string, update client.ModelsUpdateDevice) error {
	c := createClient(ctx, command)
	res := apiResponse(c.DevicesApi.
		UpdateDevice(ctx, devID).
		Update(update).
		Execute())
	show(command, deviceTableFields(command), res)
	showSuccessfully(command, "updated")
	return nil
}
