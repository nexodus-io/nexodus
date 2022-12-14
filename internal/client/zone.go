package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"

	"github.com/redhat-et/apex/internal/models"
)

const (
	ZONES = "/api/zones"
	ZONE  = "/api/zones/%s"
)

// CreateZone creates a zone
func (c *Client) CreateZone(name, description, cidr string, hubZone bool) (models.Zone, error) {
	dest := c.baseURL.JoinPath(ZONES).String()
	zone := models.AddZone{
		Name:        name,
		Description: description,
		IpCidr:      cidr,
		HubZone:     hubZone,
	}
	zoneJSON, _ := json.Marshal(zone)

	req, err := http.NewRequest(http.MethodPost, dest, bytes.NewReader(zoneJSON))
	if err != nil {
		return models.Zone{}, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return models.Zone{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Zone{}, err
	}

	if res.StatusCode != http.StatusCreated {
		return models.Zone{}, fmt.Errorf("failed to create the zone. %s", string(resBody))
	}

	var data models.Zone
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Zone{}, err
	}

	return data, nil
}

// ListZones lists all zones
func (c *Client) ListZones() ([]models.Zone, error) {
	dest := c.baseURL.JoinPath(ZONES).String()

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
		return nil, fmt.Errorf("failed to create the zone: %d", res.StatusCode)
	}

	var data []models.Zone
	if err := json.Unmarshal(resBody, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) DeleteZone(zoneID uuid.UUID) (models.Zone, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(ZONE, zoneID.String())).String()
	r, err := http.NewRequest(http.MethodDelete, dest, nil)
	if err != nil {
		return models.Zone{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.Zone{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Zone{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.Zone{}, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data models.Zone
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Zone{}, err
	}

	return data, nil
}
