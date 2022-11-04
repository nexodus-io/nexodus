package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/redhat-et/apex/internal/models"
)

const (
	USERS        = "/api/users"
	CURRENT_USER = "/api/users/me"
)

func (c *Client) GetCurrentUser() (models.User, error) {
	dest := c.baseURL.JoinPath(CURRENT_USER).String()
	r, err := http.NewRequest(http.MethodGet, dest, nil)
	if err != nil {
		return models.User{}, err
	}

	res, err := c.do(r)
	if err != nil {
		return models.User{}, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return models.User{}, err
	}

	if res.StatusCode != http.StatusOK {
		return models.User{}, fmt.Errorf("http error: %s", string(body))
	}

	var u models.User
	if err := json.Unmarshal(body, &u); err != nil {
		return models.User{}, err
	}

	return u, nil
}
