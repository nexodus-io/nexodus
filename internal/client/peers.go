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
	ZONE_PEERS = "/api/zones/%s/peers"
	PEER       = "/api/peers/%s"
)

func (c *Client) CreatePeerInZone(zoneID uuid.UUID, deviceID uuid.UUID, endpointIP string, requestedIP string, childPrefix string, hubRouter bool, hubZone bool, zonePrefix string, reflexiveIP, endpointLocalAddress string, symmetricNat bool) (models.Peer, error) {
	registerRequest := models.AddPeer{
		DeviceID:                 deviceID,
		EndpointIP:               endpointIP,
		NodeAddress:              requestedIP,
		ChildPrefix:              childPrefix,
		HubRouter:                hubRouter,
		HubZone:                  hubZone,
		ZonePrefix:               zonePrefix,
		ReflexiveIPv4:            reflexiveIP,
		EndpointLocalAddressIPv4: endpointLocalAddress,
		SymmetricNat:             symmetricNat,
	}
	body, err := json.Marshal(registerRequest)
	if err != nil {
		return models.Peer{}, err
	}

	dest := c.baseURL.JoinPath(fmt.Sprintf(ZONE_PEERS, zoneID.String())).String()
	r, err := http.NewRequest(http.MethodPost, dest, bytes.NewReader(body))
	if err != nil {
		return models.Peer{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.Peer{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Peer{}, err
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusConflict {
		return models.Peer{}, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data models.Peer
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Peer{}, err
	}

	return data, nil
}

func (c *Client) GetZonePeers(zoneID uuid.UUID) ([]models.Peer, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(ZONE_PEERS, zoneID.String())).String()
	req, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, err
	}

	var peerListing []models.Peer
	if err := json.Unmarshal(body, &peerListing); err != nil {
		return nil, err
	}

	return peerListing, nil
}

func (c *Client) DeletePeer(peerID uuid.UUID) (models.Peer, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(PEER, peerID.String())).String()
	r, err := http.NewRequest(http.MethodDelete, dest, nil)
	if err != nil {
		return models.Peer{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.Peer{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Peer{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.Peer{}, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data models.Peer
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Peer{}, err
	}

	return data, nil
}
