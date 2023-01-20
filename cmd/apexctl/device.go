package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/client"
)

func listAllDevices(c *client.Client, encodeOut string) error {
	devices, err := c.ListDevices()
	if err != nil {
		log.Fatal(err)
	}
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "DEVICE ID", "HOSTNAME", "NODE ADDRESS", "ENDPOINT IP", "PUBLIC KEY", "ZONE ID")
		}
		for _, dev := range devices {
			fmt.Fprintf(w, fs, dev.ID, dev.Hostname, dev.TunnelIP, dev.LocalIP, dev.PublicKey, dev.OrganizationID)
		}
		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, devices)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func deleteDevice(c *client.Client, encodeOut, devID string) error {
	devUUID, err := uuid.Parse(devID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", devUUID, err)
	}

	res, err := c.DeleteDevice(devUUID)
	if err != nil {
		log.Fatalf("device delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted device %s\n", res.ID.String())
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
