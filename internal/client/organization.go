package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/nexodus-io/nexodus/internal/models"
)

const (
	ORGANIZATIONS        = "/api/organizations"
	ORGANIZATION         = "/api/organizations/%s"
	ORGANIZATION_DEVICES = "/api/organizations/%s/devices"
)

// CreateOrganization creates a organization
func (c *Client) CreateOrganization(name, description, cidr string, hubZone bool) (models.OrganizationJSON, error) {
	dest := c.baseURL.JoinPath(ORGANIZATIONS).String()
	organization := models.AddOrganization{
		Name:        name,
		Description: description,
		IpCidr:      cidr,
		HubZone:     hubZone,
	}
	organizationJSON, _ := json.Marshal(organization)

	req, err := http.NewRequest(http.MethodPost, dest, bytes.NewReader(organizationJSON))
	if err != nil {
		return models.OrganizationJSON{}, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return models.OrganizationJSON{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.OrganizationJSON{}, err
	}

	if res.StatusCode != http.StatusCreated {
		return models.OrganizationJSON{}, fmt.Errorf("failed to create the organization. %s", string(resBody))
	}

	var data models.OrganizationJSON
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.OrganizationJSON{}, err
	}

	return data, nil
}

// ListOrganizations lists all organizations
func (c *Client) ListOrganizations() ([]models.OrganizationJSON, error) {
	dest := c.baseURL.JoinPath(ORGANIZATIONS).String()

	req, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list organizations %d", res.StatusCode)
	}

	var data []models.OrganizationJSON
	if err := json.Unmarshal(resBody, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// GetOrganizations gets an organization by ID
func (c *Client) GetOrganization(id uuid.UUID) (models.OrganizationJSON, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(ORGANIZATION, id.String())).String()

	req, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return models.OrganizationJSON{}, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return models.OrganizationJSON{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.OrganizationJSON{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.OrganizationJSON{}, fmt.Errorf("failed to get organization %d", res.StatusCode)
	}

	var data models.OrganizationJSON
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.OrganizationJSON{}, err
	}

	return data, nil
}

func (c *Client) GetOrganizations() ([]models.OrganizationJSON, error) {
	dest := c.baseURL.JoinPath(ORGANIZATIONS).String()

	req, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get organizations %d", res.StatusCode)
	}

	var data []models.OrganizationJSON
	if err := json.Unmarshal(resBody, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// GetOrganizations gets an organization by ID
func (c *Client) GetDeviceInOrganization(id uuid.UUID) ([]models.Device, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(ORGANIZATION_DEVICES, id.String())).String()

	req, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get organization %d", res.StatusCode)
	}

	var data []models.Device
	if err := json.Unmarshal(resBody, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) DeleteOrganization(organizationID uuid.UUID) (models.Organization, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(ORGANIZATION, organizationID.String())).String()
	r, err := http.NewRequest(http.MethodDelete, dest, nil)
	if err != nil {
		return models.Organization{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.Organization{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Organization{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.Organization{}, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data models.Organization
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Organization{}, err
	}

	return data, nil
}
