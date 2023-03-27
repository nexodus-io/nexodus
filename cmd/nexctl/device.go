package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
)

func listOrgDevices(c *client.Client, organizationID uuid.UUID, encodeOut string) error {
	devices, err := c.GetDeviceInOrganization(organizationID)
	if err != nil {
		log.Fatal(err)
	}
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "DEVICE ID", "HOSTNAME", "NODE ADDRESS IPV4", "NODE ADDRESS IPV6", "ENDPOINT IP", "PUBLIC KEY", "ORGANIZATION ID", "RELAY")
		}
		for _, dev := range devices {
			fmt.Fprintf(w, fs, dev.ID, dev.Hostname, dev.TunnelIP, dev.TunnelIpV6, dev.LocalIP, dev.PublicKey, dev.OrganizationID, fmt.Sprintf("%t", dev.Relay))
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

func listAllDevices(c *client.Client, encodeOut string) error {
	devices, err := c.ListDevices()
	if err != nil {
		log.Fatal(err)
	}
	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "DEVICE ID", "HOSTNAME", "NODE ADDRESS",
				"ENDPOINT IP", "PUBLIC KEY", "ORGANIZATION ID",
				"LOCAL IP", "ALLOWED IPS", "TUNNEL IPV4", "TUNNEL IPV6",
				"CHILD PREFIX", "ORG PREFIX IPV4", "ORG PREFIX IPV6",
				"REFLEXIVE IPv4", "ENDPOINT LOCAL IPv4", "RELAY")
		}
		for _, dev := range devices {
			fmt.Fprintf(w, fs, dev.ID, dev.Hostname, dev.TunnelIP, dev.LocalIP, dev.PublicKey, dev.OrganizationID,
				dev.LocalIP, dev.AllowedIPs, dev.TunnelIP, dev.TunnelIpV6, dev.ChildPrefix, dev.OrganizationPrefix,
				dev.OrganizationPrefixV6, dev.ReflexiveIPv4, dev.EndpointLocalAddressIPv4, fmt.Sprintf("%t", dev.Relay))
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
