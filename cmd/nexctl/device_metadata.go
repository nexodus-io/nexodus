package main

import (
	"github.com/urfave/cli/v2"
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
			Action: func(c *cli.Context) error {
				orgId, err := getUUID(c, "vpc-id")
				if err != nil {
					return err
				}
				return getVpcMetadata(c, orgId)
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
			Action: func(ctx *cli.Context) error {
				deviceID, err := getUUID(ctx, "device-id")
				if err != nil {
					return err
				}
				if ctx.IsSet("key") {
					return getDeviceMetadataKey(ctx, deviceID, ctx.String("key"))
				} else {
					return getDeviceMetadata(ctx, deviceID)
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
			Action: func(ctx *cli.Context) error {
				deviceID, err := getUUID(ctx, "device-id")
				if err != nil {
					return err
				}
				value, err := getJsonMap(ctx, "value")
				if err != nil {
					return err
				}
				return updateDeviceMetadata(ctx, deviceID, ctx.String("key"), value)
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
			Action: func(ctx *cli.Context) error {
				deviceID, err := getUUID(ctx, "device-id")
				if err != nil {
					return err
				}
				return deleteDeviceMetadata(ctx, deviceID, ctx.String("key"))
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
			Action: func(ctx *cli.Context) error {
				deviceID, err := getUUID(ctx, "device-id")
				if err != nil {
					return err
				}
				return clearDeviceMetadata(ctx, deviceID)
			},
		},
	}
}

func metadataTableFields(cCtx *cli.Context, includeDeviceId bool) []TableField {
	var fields = []TableField{}
	full := cCtx.Bool("full")
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

func getDeviceMetadata(ctx *cli.Context, deviceID string) error {
	c := createClient(ctx)
	res := apiResponse(c.DevicesApi.
		ListDeviceMetadata(ctx.Context, deviceID).
		Execute())
	show(ctx, metadataTableFields(ctx, false), res)
	return nil
}

func getDeviceMetadataKey(ctx *cli.Context, deviceID string, key string) error {
	c := createClient(ctx)
	res := apiResponse(c.DevicesApi.
		GetDeviceMetadataKey(ctx.Context, deviceID, key).
		Execute())
	show(ctx, metadataTableFields(ctx, false), res)
	return nil
}

func getVpcMetadata(ctx *cli.Context, vpcID string) error {
	c := createClient(ctx)
	prefixes := []string{}
	res := apiResponse(c.VPCApi.
		ListMetadataInVPC(ctx.Context, vpcID, prefixes).
		Execute())
	show(ctx, metadataTableFields(ctx, true), res)
	return nil
}

func updateDeviceMetadata(ctx *cli.Context, deviceID string, key string, value map[string]interface{}) error {
	c := createClient(ctx)
	res := apiResponse(c.DevicesApi.
		UpdateDeviceMetadataKey(ctx.Context, deviceID, key).
		Value(value).
		Execute())
	show(ctx, metadataTableFields(ctx, false), res)
	return nil
}

func deleteDeviceMetadata(ctx *cli.Context, deviceID string, key string) error {
	c := createClient(ctx)
	httpResp, err := c.DevicesApi.
		DeleteDeviceMetadataKey(ctx.Context, deviceID, key).
		Execute()
	_ = apiResponse("", httpResp, err)
	showSuccessfully(ctx, "deleted")
	return nil
}

func clearDeviceMetadata(ctx *cli.Context, deviceID string) error {
	c := createClient(ctx)
	httpResp, err := c.DevicesApi.
		DeleteDeviceMetadata(ctx.Context, deviceID).
		Execute()
	_ = apiResponse("", httpResp, err)
	return nil
}
