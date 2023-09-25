package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/urfave/cli/v2"
)

const LocalTimeFormat = "2006-01-02 15:04:05 MST"

func listOrgDevices(cCtx *cli.Context, c *public.APIClient, organizationID uuid.UUID) error {
	devices, _, err := c.DevicesApi.ListDevicesInOrganization(context.Background(), organizationID.String()).Execute()
	if err != nil {
		log.Fatal(err)
	}
	showOutput(cCtx, deviceTableFields(cCtx), devices)
	return nil
}

func deviceTableFields(cCtx *cli.Context) []TableField {
	var fields []TableField
	full := cCtx.Bool("full")

	fields = append(fields, TableField{Header: "DEVICE ID", Field: "Id"})
	fields = append(fields, TableField{Header: "HOSTNAME", Field: "Hostname"})
	if full {
		fields = append(fields, TableField{Header: "TUNNEL IPV4", Field: "TunnelIp"})
		fields = append(fields, TableField{Header: "TUNNEL IPV6", Field: "TunnelIpV6"})
	} else {
		fields = append(fields, TableField{Header: "TUNNEL IPS",
			Formatter: func(item interface{}) string {
				dev := item.(public.ModelsDevice)
				return fmt.Sprintf("%s, %s", dev.TunnelIp, dev.TunnelIpV6)
			},
		})
	}

	fields = append(fields, TableField{Header: "ORGANIZATION ID", Field: "OrganizationId"})
	fields = append(fields, TableField{Header: "RELAY", Field: "Relay"})
	if full {
		fields = append(fields, TableField{Header: "PUBLIC KEY", Field: "PublicKey"})
		fields = append(fields, TableField{
			Header: "LOCAL IP",
			Formatter: func(item interface{}) string {
				dev := item.(public.ModelsDevice)
				for _, endpoint := range dev.Endpoints {
					if endpoint.Source == "local" {
						return endpoint.Address
					}
				}
				return ""
			},
		})
		fields = append(fields, TableField{Header: "ADVERTISED CIDR",
			Formatter: func(item interface{}) string {
				dev := item.(public.ModelsDevice)
				return strings.Join(dev.AllowedIps, ", ")
			},
		})
		fields = append(fields, TableField{Header: "ORG PREFIX IPV6", Field: "OrganizationPrefixV6"})
		fields = append(fields, TableField{Header: "REFLEXIVE IPv4",
			Formatter: func(item interface{}) string {
				dev := item.(public.ModelsDevice)
				var reflexiveIp4 []string
				for _, endpoint := range dev.Endpoints {
					if endpoint.Source != "local" {
						reflexiveIp4 = append(reflexiveIp4, endpoint.Address)
					}
				}
				return strings.Join(reflexiveIp4, ", ")
			},
		})
		fields = append(fields, TableField{Header: "ENDPOINT LOCAL IPv4", Field: "EndpointLocalAddressIp4"})
		fields = append(fields, TableField{Header: "OS", Field: "Os"})
		fields = append(fields, TableField{Header: "SECURITY GROUP ID", Field: "SecurityGroupId"})
		fields = append(fields, TableField{Header: "ONLINE", Field: "Online"})
		fields = append(fields, TableField{Header: "ONLINE SINCE", Field: "OnlineAt"})
	}
	return fields
}

func listAllDevices(cCtx *cli.Context, c *public.APIClient) error {
	devices, _, err := c.DevicesApi.ListDevices(context.Background()).Execute()
	if err != nil {
		log.Fatal(err)
	}

	// Only modify the time to a user-friendly value if the nexctl output is in column form
	if cCtx.String("output") == encodeColumn || cCtx.String("output") == encodeNoHeader {
		for i := range devices {
			convertDeviceTimeToLocal(&devices[i])
		}
	}

	showOutput(cCtx, deviceTableFields(cCtx), devices)
	return nil
}

func deleteDevice(c *public.APIClient, encodeOut, devID string) error {
	devUUID, err := uuid.Parse(devID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", devUUID, err)
	}

	res, _, err := c.DevicesApi.DeleteDevice(context.Background(), devUUID.String()).Execute()
	if err != nil {
		log.Fatalf("device delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted device %s\n", res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

// convertToLocalTime converts the OnlineAt field to local time
func convertDeviceTimeToLocal(d *public.ModelsDevice) {
	if !d.Online {
		// If the device is not online, set the time field to empty
		d.OnlineAt = ""
		return
	}

	// Try to parse the time and convert it to local time
	parsedTime, err := time.Parse(time.RFC3339, d.OnlineAt)
	if err != nil {
		return
	}

	localTime := parsedTime.Local()
	d.OnlineAt = localTime.Format(LocalTimeFormat)
}
