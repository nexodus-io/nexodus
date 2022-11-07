package client

import (
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	baseURL *url.URL
	auth    Authenticator
	client  *http.Client
}

func NewClient(addr string, auth Authenticator) (Client, error) {
	baseURL, err := url.Parse(addr)
	if err != nil {
		return Client{}, err
	}
	return Client{
		baseURL: baseURL,
		auth:    auth,
		client:  http.DefaultClient,
	}, nil
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	accessToken, err := c.auth.Token()
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", fmt.Sprintf("bearer %s", accessToken))
	return c.client.Do(req)
}
