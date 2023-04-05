package main

import (
	"context"
	"fmt"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"log"

	"github.com/google/uuid"
)

func listOrgDevices(c *public.APIClient, organizationID uuid.UUID, encodeOut string) error {
	devices, _, err := c.DevicesApi.ListDevicesInOrganization(context.Background(), organizationID.String()).Execute()
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
			fmt.Fprintf(w, fs, dev.Id, dev.Hostname, dev.TunnelIp, dev.TunnelIpV6, dev.LocalIp, dev.PublicKey, dev.OrganizationId, fmt.Sprintf("%t", dev.Relay))
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

func listAllDevices(c *public.APIClient, encodeOut string) error {
	devices, _, err := c.DevicesApi.ListDevices(context.Background()).Execute()
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
			fmt.Fprintf(w, fs, dev.Id, dev.Hostname, dev.TunnelIp, dev.LocalIp, dev.PublicKey, dev.OrganizationId,
				dev.LocalIp, dev.AllowedIps, dev.TunnelIp, dev.TunnelIpV6, dev.ChildPrefix, dev.OrganizationPrefix,
				dev.OrganizationPrefixV6, dev.ReflexiveIp4, dev.EndpointLocalAddressIp4, fmt.Sprintf("%t", dev.Relay))
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
