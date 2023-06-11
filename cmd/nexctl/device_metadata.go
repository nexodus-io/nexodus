package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
)

var deviceMetadataSubcommands []*cli.Command

func init() {
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
			},
			Action: func(c *cli.Context) error {
				deviceID, err := uuid.Parse(c.String("device-id"))
				if err != nil {
					return err
				}
				key := c.String("key")

				if deviceID != uuid.Nil && key != "" {
					return getDeviceMetadataKey(c, deviceID, key)
				} else if deviceID != uuid.Nil {
					return getDeviceMetadata(c, deviceID)
				} else {
					return fmt.Errorf("device-id is required")
				}
			},
		},
		{
			Name:  "update",
			Usage: "Update device metadata",
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

func getDeviceMetadata(c *cli.Context, deviceID uuid.UUID) error {
	client := mustCreateAPIClient(c)

	// validate device ID
	_, _, err := client.DevicesApi.GetDevice(context.Background(), deviceID.String()).Execute()
	if err != nil {
		return err
	}

	metadata, _, err := client.DevicesApi.GetDeviceMetadata(context.Background(), deviceID.String()).Execute()
	if err != nil {
		return err
	}

	fmt.Println(metadata)

	return nil
}

func getDeviceMetadataKey(c *cli.Context, deviceID uuid.UUID, key string) error {
	return fmt.Errorf("not implemented")
}

func updateDeviceMetadata(c *cli.Context, deviceID uuid.UUID, key, value string) error {
	return fmt.Errorf("not implemented")
}

func deleteDeviceMetadata(c *cli.Context, deviceID uuid.UUID, key string) error {
	return fmt.Errorf("not implemented")
}

func clearDeviceMetadata(c *cli.Context, deviceID uuid.UUID) error {
	return fmt.Errorf("not implemented")
}
