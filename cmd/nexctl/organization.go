package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/api/public"
	"github.com/nexodus-io/nexodus/internal/client"
)

func listOrganizations(c *client.APIClient, encodeOut string) error {
	orgs, _, err := c.OrganizationsApi.ListOrganizations(context.Background()).Execute()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		w := newTabWriter()
		fs := "%s\t%s\t%s\t%s\t%s\t%s\n"
		if encodeOut != encodeNoHeader {
			fmt.Fprintf(w, fs, "Organization ID", "NAME", "IPV4 CIDR", "IPV6 CIDR", "DESCRIPTION", "SECURITY GROUP ID")
		}

		for _, org := range orgs {
			fmt.Fprintf(w, fs, org.Id, org.Name, org.Cidr, org.CidrV6, org.Description, org.SecurityGroupId)
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

func createOrganization(c *client.APIClient, encodeOut, name, description, cidr string, cidrV6 string, hub bool) error {
	res, _, err := c.OrganizationsApi.CreateOrganization(context.Background()).Organization(public.ModelsAddOrganization{
		Name:        name,
		Description: description,
		Cidr:        cidr,
		CidrV6:      cidrV6,
		HubZone:     hub,
		PrivateCidr: !(cidr == "" && cidrV6 == ""),
	}).Execute()
	if err != nil {
		log.Fatal(err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Println(res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}

/*
func moveUserToOrganization(c *client.APIClient, encodeOut, username, OrganizationID string) error {
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

func deleteOrganization(c *client.APIClient, encodeOut, OrganizationID string) error {
	OrganizationUUID, err := uuid.Parse(OrganizationID)
	if err != nil {
		log.Fatalf("failed to parse a valid UUID from %s %v", OrganizationUUID, err)
	}

	res, _, err := c.OrganizationsApi.DeleteOrganization(context.Background(), OrganizationUUID.String()).Execute()
	if err != nil {
		log.Fatalf("Organization delete failed: %v\n", err)
	}

	if encodeOut == encodeColumn || encodeOut == encodeNoHeader {
		fmt.Printf("successfully deleted Organization %s\n", res.Id)
		return nil
	}

	err = FormatOutput(encodeOut, res)
	if err != nil {
		log.Fatalf("failed to print output: %v", err)
	}

	return nil
}
