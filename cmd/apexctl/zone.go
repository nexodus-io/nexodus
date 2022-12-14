package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/client"
)

func listZones(c *client.Client, encodeOut string) error {
	zones, err := c.ListZones()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "ZONE ID", "NAME", "CIDR", "DESCRIPTION", "RELAY/HUB ENABLED")
		}

		for _, zone := range zones {
			fmt.Fprintf(w, fs, zone.ID, zone.Name, zone.IpCidr, zone.Description, strconv.FormatBool(zone.HubZone))
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, zones)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func createZone(c *client.Client, encodeOut, name, description, cidr string, hub bool) error {
	res, err := c.CreateZone(name, description, cidr, hub)
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println(res.ID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func moveUserToZone(c *client.Client, encodeOut, username, zoneID string) error {
	zoneUUID, err := uuid.Parse(zoneID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", zoneID, err)
	}

	res, err := c.MoveCurrentUserToZone(zoneUUID)
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("%s successfully moved into zone %s\n", username, zoneID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func deleteZone(c *client.Client, encodeOut, zoneID string) error {
	zoneUUID, err := uuid.Parse(zoneID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", zoneUUID, err)
	}

	res, err := c.DeleteZone(zoneUUID)
	if err != nil {
		log.Fatalf("zone delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted zone %s\n", res.ID.String())
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
