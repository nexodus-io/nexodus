package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/redhat-et/apex/internal/models"
)

const (
	DEVICES = "/api/devices"
	DEVICE  = "/api/devices/%s"
)

func (c *Client) CreateDevice(publicKey string, hostname string) (models.Device, error) {
	body, err := json.Marshal(models.AddDevice{
		PublicKey: publicKey,
		Hostname:  hostname,
	})
	if err != nil {
		return models.Device{}, err
	}

	dest := c.baseURL.JoinPath(DEVICES).String()
	r, err := http.NewRequest(http.MethodPost, dest, bytes.NewReader(body))
	if err != nil {
		return models.Device{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.Device{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Device{}, err
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusConflict {
		return models.Device{}, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data models.Device
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Device{}, err
	}

	return data, nil
}

func (c *Client) GetDevice(deviceID uuid.UUID) (models.Device, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(DEVICE, deviceID.String())).String()
	r, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return models.Device{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.Device{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Device{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.Device{}, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data models.Device
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Device{}, err
	}

	return data, nil
}
