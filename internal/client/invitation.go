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
	INVITATIONS       = "/api/invitations"
	INVITATION        = "/api/invitations/%s"
	INVITATION_ACCEPT = "/api/invitations/%s/accept"
)

// CreateInvitation creates an invitation
func (c *Client) CreateInvitation(userID string, orgID uuid.UUID) (models.Invitation, error) {
	dest := c.baseURL.JoinPath(INVITATIONS).String()
	organization := models.AddInvitation{
		UserID:         userID,
		OrganizationID: orgID,
	}
	inviteJSON, _ := json.Marshal(organization)

	req, err := http.NewRequest(http.MethodPost, dest, bytes.NewReader(inviteJSON))
	if err != nil {
		return models.Invitation{}, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return models.Invitation{}, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.Invitation{}, err
	}

	if res.StatusCode != http.StatusCreated {
		return models.Invitation{}, fmt.Errorf("failed to create the invitation. %s", string(resBody))
	}

	var data models.Invitation
	if err := json.Unmarshal(resBody, &data); err != nil {
		return models.Invitation{}, err
	}

	return data, nil
}

// AcceptInvitation accepts and invitation
func (c *Client) AcceptInvitation(id uuid.UUID) error {
	dest := c.baseURL.JoinPath(fmt.Sprintf(INVITATION_ACCEPT, id.String())).String()

	req, err := http.NewRequest(http.MethodPost, dest, nil)
	if err != nil {
		return err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to accept invitation %d", res.StatusCode)
	}
	return nil
}

// DeleteInvitation deletes an invitation
func (c *Client) DeleteInvitation(id uuid.UUID) error {
	dest := c.baseURL.JoinPath(fmt.Sprintf(INVITATION_ACCEPT, id.String())).String()

	req, err := http.NewRequest(http.MethodDelete, dest, nil)
	if err != nil {
		return err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to accept invitation %d", res.StatusCode)
	}
	return nil
}
