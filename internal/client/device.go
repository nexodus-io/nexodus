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
	DEVICES = "/api/devices"
	DEVICE  = "/api/devices/%s"
)

type ErrConflict struct {
	ID string
}

func (e ErrConflict) Error() string {
	return fmt.Sprintf("resource with ID %s conflicts with request resource", e.ID)
}

func (c *Client) CreateDevice(device models.AddDevice) (models.Device, error) {
	body, err := json.Marshal(device)
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

	if res.StatusCode == http.StatusConflict {
		var data models.ConflictsError
		if err := json.Unmarshal(resBody, &data); err != nil {
			return models.Device{}, err
		}
		return models.Device{}, ErrConflict{ID: data.ID}
	}

	if res.StatusCode != http.StatusCreated {
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

func (c *Client) UpdateDevice(deviceID uuid.UUID, update models.UpdateDevice) (models.Device, error) {
	body, err := json.Marshal(update)
	if err != nil {
		return models.Device{}, err
	}
	dest := c.baseURL.JoinPath(fmt.Sprintf(DEVICE, deviceID.String())).String()
	r, err := http.NewRequest(http.MethodPatch, dest, bytes.NewReader(body))
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

func (c *Client) ListDevices() ([]models.Device, error) {
	dest := c.baseURL.JoinPath(DEVICES).String()
	r, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data []models.Device
	if err := json.Unmarshal(resBody, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) DeleteDevice(deviceID uuid.UUID) (models.Device, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(DEVICE, deviceID.String())).String()
	r, err := http.NewRequest(http.MethodDelete, dest, nil)
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
