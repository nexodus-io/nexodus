package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nexodus-io/nexodus/internal/models"
)

const (
	USERS                = "/api/users"
	USER                 = "/api/users/%s"
	DELETE_USER_FROM_ORG = "/api/users/%s/organizations/%s"
	CURRENT_USER         = "/api/users/me"
)

func (c *Client) GetCurrentUser() (models.UserJSON, error) {
	dest := c.baseURL.JoinPath(CURRENT_USER).String()
	r, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return models.UserJSON{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.UserJSON{}, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return models.UserJSON{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.UserJSON{}, fmt.Errorf("http error: %s", string(body))
	}

	var u models.UserJSON
	if err := json.Unmarshal(body, &u); err != nil {
		return models.UserJSON{}, err
	}

	return u, nil
}

/* MoveCurrentUserToZone moves the current user into a given zone
func (c *Client) MoveCurrentUserToOrganization(zoneID uuid.UUID) (models.User, error) {
	dest := c.baseURL.JoinPath(CURRENT_USER).String()
	user := models.PatchUser{
		Organizations: zoneID,
	}
	userJSON, _ := json.Marshal(user)

	req, err := http.NewRequest(http.MethodPatch, dest, bytes.NewBuffer(userJSON))
	if err != nil {
		return models.UserJSON{}, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return models.UserJSON{}, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return models.UserJSON{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.UserJSON{}, fmt.Errorf("failed to patch the user into the zone: %s", zoneID)
	}

	var u models.User
	if err := json.Unmarshal(body, &u); err != nil {
		return models.UserJSON{}, err
	}

	return u, nil
}
*/

func (c *Client) ListUsers() ([]models.UserJSON, error) {
	dest := c.baseURL.JoinPath(USERS).String()

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
		return nil, fmt.Errorf("failed to list users: %d", res.StatusCode)
	}

	var data []models.UserJSON
	if err := json.Unmarshal(resBody, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// DeleteUser deletes a user
func (c *Client) DeleteUser(userID string) (models.UserJSON, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(USER, userID)).String()
	r, err := http.NewRequest(http.MethodDelete, dest, nil)
	if err != nil {
		return models.UserJSON{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.UserJSON{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return models.UserJSON{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.UserJSON{}, fmt.Errorf("http error: %s", string(body))
	}

	var u models.UserJSON
	if err := json.Unmarshal(body, &u); err != nil {
		return models.UserJSON{}, err
	}

	return u, nil
}

func (c *Client) DeleteUserFromOrganization(userID, organizationID string) (models.UserJSON, error) {
	dest := c.baseURL.JoinPath(fmt.Sprintf(DELETE_USER_FROM_ORG, userID, organizationID)).String()
	r, err := http.NewRequest(http.MethodDelete, dest, nil)
	if err != nil {
		return models.UserJSON{}, err
	}

	res, err := c.client.Do(r)
	if err != nil {
		return models.UserJSON{}, err
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return models.UserJSON{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.UserJSON{}, fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var u models.UserJSON
	if err := json.Unmarshal(resBody, &u); err != nil {
		return models.UserJSON{}, err
	}

	return u, nil
}
