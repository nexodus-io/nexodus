package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/client"
	"log"
)

func listOrganizations(c *client.Client, encodeOut string) error {
	orgs, err := c.ListOrganizations()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "Organization ID", "NAME", "IPV4 CIDR", "IPV6 CIDR", "DESCRIPTION")
		}

		for _, org := range orgs {
			fmt.Fprintf(w, fs, org.ID, org.Name, org.IpCidr, org.IpCidrV6, org.Description)
		}

		w.Flush()

		return nil
	}

	err = FormatOutput(encodeOut, orgs)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

func createOrganization(c *client.Client, encodeOut, name, description, cidr string, cidrV6 string, hub bool) error {
	res, err := c.CreateOrganization(name, description, cidr, cidrV6, hub)
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

/*
func moveUserToOrganization(c *client.Client, encodeOut, username, OrganizationID string) error {
	OrganizationUUID, err := uuid.Parse(OrganizationID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", OrganizationID, err)
	}

	res, err := c.MoveCurrentUserToOrganization(OrganizationUUID)
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("%s successfully moved into Organization %s\n", username, OrganizationID)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
*/

func deleteOrganization(c *client.Client, encodeOut, OrganizationID string) error {
	OrganizationUUID, err := uuid.Parse(OrganizationID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", OrganizationUUID, err)
	}

	res, err := c.DeleteOrganization(OrganizationUUID)
	if err != nil {
		log.Fatalf("Organization delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted Organization %s\n", res.ID.String())
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
