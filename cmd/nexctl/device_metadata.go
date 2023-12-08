package main

import (
	"context"
	"github.com/urfave/cli/v3"
)

var deviceMetadataSubcommands []*cli.Command
var vpcMetadataSubcommands []*cli.Command

func init() {
	vpcMetadataSubcommands = []*cli.Command{
		{
			Name:  "get",
			Usage: "Get device metadata",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "vpc-id",
					Required: true,
				},
				&cli.BoolFlag{
					Name:    "full",
					Aliases: []string{"f"},
					Usage:   "display the full set of metadata details",
					Value:   false,
				},
			},
			Action: func(ctx context.Context, command *cli.Command) error {
				orgId, err := getUUID(command, "vpc-id")
				if err != nil {
					return err
				}
				return getVpcMetadata(ctx, command, orgId)
			},
		},
	}
	deviceMetadataSubcommands = []*cli.Command{
		{
			Name:  "get",
			Usage: "Get device metadata",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "device-id",
					Usage:    "Device ID",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "key",
					Usage:    "Metadata Key",
					Required: false,
				},
				&cli.BoolFlag{
					Name:    "full",
					Aliases: []string{"f"},
					Usage:   "display the full set of metadata details",
					Value:   false,
				},
			},
			Action: func(ctx context.Context, command *cli.Command) error {
				deviceID, err := getUUID(command, "device-id")
				if err != nil {
					return err
				}
				if command.IsSet("key") {
					return getDeviceMetadataKey(ctx, command, deviceID, command.String("key"))
				} else {
					return getDeviceMetadata(ctx, command, deviceID)
				}
			},
		},

		{
			Name:  "set",
			Usage: "Set device metadata",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "device-id",
					Usage:    "Device ID",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "key",
					Usage:    "Metadata Key",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "value",
					Usage:    "Metadata Value",
					Required: true,
				},
				&cli.BoolFlag{
					Name:    "full",
					Aliases: []string{"f"},
					Usage:   "display the full set of metadata details",
					Value:   false,
				},
			},
			Action: func(ctx context.Context, command *cli.Command) error {
				deviceID, err := getUUID(command, "device-id")
				if err != nil {
					return err
				}
				value, err := getJsonMap(command, "value")
				if err != nil {
					return err
				}
				return updateDeviceMetadata(ctx, command, deviceID, command.String("key"), value)
			},
		},
		{
			Name:  "delete",
			Usage: "Delete device metadata",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "device-id",
					Usage:    "Device ID",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "key",
					Usage:    "Metadata Key",
					Required: true,
				},
			},
			Action: func(ctx context.Context, command *cli.Command) error {
				deviceID, err := getUUID(command, "device-id")
				if err != nil {
					return err
				}
				return deleteDeviceMetadata(ctx, command, deviceID, command.String("key"))
			},
		},
		{
			Name:  "clear",
			Usage: "Clear all device metadata",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "device-id",
					Usage:    "Device ID",
					Required: true,
				},
			},
			Action: func(ctx context.Context, command *cli.Command) error {
				deviceID, err := getUUID(command, "device-id")
				if err != nil {
					return err
				}
				return clearDeviceMetadata(ctx, command, deviceID)
			},
		},
	}
}

func metadataTableFields(command *cli.Command, includeDeviceId bool) []TableField {
	var fields = []TableField{}
	full := command.Bool("full")
	if includeDeviceId || full {
		fields = append(fields, TableField{Header: "DEVICE ID", Field: "DeviceId"})
	}
	fields = append(fields, TableField{Header: "KEY", Field: "Key"})
	fields = append(fields, TableField{Header: "VALUE", Field: "Value"})
	if full {
		fields = append(fields, TableField{Header: "REVISION", Field: "Revision"})
	}
	return fields
}

func getDeviceMetadata(ctx context.Context, command *cli.Command, deviceID string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.DevicesApi.
		ListDeviceMetadata(ctx, deviceID).
		Execute())
	show(command, metadataTableFields(command, false), res)
	return nil
}

func getDeviceMetadataKey(ctx context.Context, command *cli.Command, deviceID string, key string) error {
	c := createClient(ctx, command)
	res := apiResponse(c.DevicesApi.
		GetDeviceMetadataKey(ctx, deviceID, key).
		Execute())
	show(command, metadataTableFields(command, false), res)
	return nil
}

func getVpcMetadata(ctx context.Context, command *cli.Command, vpcID string) error {
	c := createClient(ctx, command)
	prefixes := []string{}
	res := apiResponse(c.VPCApi.
		ListMetadataInVPC(ctx, vpcID, prefixes).
		Execute())
	show(command, metadataTableFields(command, true), res)
	return nil
}

func updateDeviceMetadata(ctx context.Context, command *cli.Command, deviceID string, key string, value map[string]interface{}) error {
	c := createClient(ctx, command)
	res := apiResponse(c.DevicesApi.
		UpdateDeviceMetadataKey(ctx, deviceID, key).
		Value(value).
		Execute())
	show(command, metadataTableFields(command, false), res)
	return nil
}

func deleteDeviceMetadata(ctx context.Context, command *cli.Command, deviceID string, key string) error {
	c := createClient(ctx, command)
	httpResp, err := c.DevicesApi.
		DeleteDeviceMetadataKey(ctx, deviceID, key).
		Execute()
	_ = apiResponse("", httpResp, err)
	showSuccessfully(command, "deleted")
	return nil
}

func clearDeviceMetadata(ctx context.Context, command *cli.Command, deviceID string) error {
	c := createClient(ctx, command)
	httpResp, err := c.DevicesApi.
		DeleteDeviceMetadata(ctx, deviceID).
		Execute()
	_ = apiResponse("", httpResp, err)
	return nil
}
