package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/client"
	"github.com/redhat-et/apex/internal/models"
)

func listAllDevices(c *client.Client, encodeOut string) error {
	allPeers := getAllPeers(c)

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "DEVICE ID", "HOSTNAME", "NODE ADDRESS", "ENDPOINT IP", "PUBLIC KEY", "ZONE ID")
		}

		for _, peer := range allPeers {
			dev, err := c.GetDevice(peer.DeviceID)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(w, fs, dev.ID, dev.Hostname, peer.NodeAddress, peer.EndpointIP, dev.PublicKey, peer.ZoneID)
		}

		w.Flush()

		return nil
	}

	var devices []models.Device
	for _, peer := range allPeers {
		dev, err := c.GetDevice(peer.DeviceID)
		devices = append(devices, dev)
		if err != nil {
			log.Fatal(err)
		}
	}

	err := FormatOutput(encodeOut, devices)
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
