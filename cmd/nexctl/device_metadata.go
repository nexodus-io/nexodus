package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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
				orgId, err := uuid.Parse(c.String("vpc-id"))
				if err != nil {
					return fmt.Errorf("invalid vpc-id: %w", err)
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
			Action: func(c *cli.Context) error {
				deviceID, err := uuid.Parse(c.String("device-id"))
				if err != nil {
					return fmt.Errorf("invalid device-id: %w", err)
				}
				if c.IsSet("key") {
					return getDeviceMetadataKey(c, deviceID, c.String("key"))
				} else {
					return getDeviceMetadata(c, deviceID)
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
			Action: func(c *cli.Context) error {
				deviceID, err := uuid.Parse(c.String("device-id"))
				if err != nil {
					return err
				}
				key := c.String("key")
				value := c.String("value")

				if deviceID != uuid.Nil && key != "" && value != "" {
					return updateDeviceMetadata(c, deviceID, key, value)
				} else {
					return fmt.Errorf("device-id, key and value are required")
				}
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
			Action: func(c *cli.Context) error {
				deviceID, err := uuid.Parse(c.String("device-id"))
				if err != nil {
					return err
				}
				key := c.String("key")

				if deviceID != uuid.Nil && key != "" {
					return deleteDeviceMetadata(c, deviceID, key)
				} else {
					return fmt.Errorf("device-id and key are required")
				}
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
			Action: func(c *cli.Context) error {
				deviceID, err := uuid.Parse(c.String("device-id"))
				if err != nil {
					return err
				}

				if deviceID != uuid.Nil {
					return clearDeviceMetadata(c, deviceID)
				} else {
					return fmt.Errorf("device-id is required")
				}
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

func getDeviceMetadata(c *cli.Context, deviceID uuid.UUID) error {
	client := mustCreateAPIClient(c)

	metadata, _, err := client.DevicesApi.
		ListDeviceMetadata(context.Background(), deviceID.String()).
		Execute()
	if err != nil {
		return err
	}

	showOutput(c, metadataTableFields(c, false), metadata)

	return nil
}

func getDeviceMetadataKey(c *cli.Context, deviceID uuid.UUID, key string) error {
	client := mustCreateAPIClient(c)

	metadata, _, err := client.DevicesApi.
		GetDeviceMetadataKey(context.Background(), deviceID.String(), key).
		Execute()
	if err != nil {
		return err
	}

	showOutput(c, metadataTableFields(c, false), metadata)
	return nil
}

func getVpcMetadata(c *cli.Context, vpcID uuid.UUID) error {
	client := mustCreateAPIClient(c)

	prefixes := []string{}
	metadata, _, err := client.VPCApi.
		ListMetadataInVPC(context.Background(), vpcID.String(), prefixes).
		Execute()
	if err != nil {
		return err
	}

	showOutput(c, metadataTableFields(c, true), metadata)

	return nil
}

func updateDeviceMetadata(c *cli.Context, deviceID uuid.UUID, key, value string) error {

	valueMap := map[string]interface{}{}
	err := json.Unmarshal([]byte(value), &valueMap)
	if err != nil {
		return fmt.Errorf("value must be a json obejct: %w", err)
	}

	client := mustCreateAPIClient(c)

	metadata, _, err := client.DevicesApi.
		UpdateDeviceMetadataKey(context.Background(), deviceID.String(), key).
		Value(valueMap).
		Execute()
	if err != nil {
		return err
	}

	showOutput(c, metadataTableFields(c, false), metadata)
	return nil
}

func deleteDeviceMetadata(c *cli.Context, deviceID uuid.UUID, key string) error {
	client := mustCreateAPIClient(c)

	_, err := client.DevicesApi.
		DeleteDeviceMetadataKey(context.Background(), deviceID.String(), key).
		Execute()
	if err != nil {
		return err
	}

	return nil
}

func clearDeviceMetadata(c *cli.Context, deviceID uuid.UUID) error {
	client := mustCreateAPIClient(c)

	_, err := client.DevicesApi.
		DeleteDeviceMetadata(context.Background(), deviceID.String()).
		Execute()
	if err != nil {
		return err
	}
	return nil
}

type TableField struct {
	Header    string
	Field     string
	Formatter func(item interface{}) string
}
