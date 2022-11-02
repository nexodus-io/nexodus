package apex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// TODO: These consts witll differ from installation to installation.
// Need to find a way to provide these dynamically (config file) etc...
const (
	APEX_CLIENT_ID     = "apex-cli"
	APEX_CLIENT_SECRET = "QkskUDQenfXRxWx9UA0TeuwmOnHilHtQ"
	LOGIN_URL          = "/auth/realms/controller/protocol/openid-connect/auth/device"
	VERIFICATION_URI   = "/auth/realms/controller/device"
	VERIFY_URL         = "/auth/realms/controller/protocol/openid-connect/token"
	GRANT_TYPE         = "urn:ietf:params:oauth:grant-type:device_code"
	REGISTER_DEVICE    = "/api/devices"
	USER_URL           = "/api/users/me"
)

type TokenResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type Authenticator struct {
	hostname     *url.URL
	accessToken  string
	refreshToken string
}

func NewAuthenticator(hostname *url.URL) Authenticator {
	return Authenticator{
		hostname:     hostname,
		accessToken:  "",
		refreshToken: "",
	}
}

func (a *Authenticator) Token() (string, error) {
	// TODO: We should handle using the refreshToken if the
	// current access token has expired.
	if a.accessToken != "" {
		return a.accessToken, nil
	}
	return "", fmt.Errorf("not authenticated")
}

func (a *Authenticator) Authenticate(ctx context.Context) error {
	token, err := getToken(a.hostname)
	if err != nil {
		return err
	}

	fmt.Println("Your device must be registered with Apex Controller.")
	fmt.Printf("Your one-time code is: %s\n", token.UserCode)
	fmt.Println("Please open the following URL in your browser and enter your one-time code:")
	dest, err := url.JoinPath(a.hostname.String(), VERIFICATION_URI)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", dest)

	c := make(chan error, 1)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(token.ExpiresIn)*time.Second)
	defer cancel()
	go func() {
		c <- a.pollForResponse(ctx, token)
	}()

	err = <-c
	if err != nil {
		return err
	}
	fmt.Println("Authentication succeeded.")
	return nil
}

func getToken(hostname *url.URL) (*TokenResponse, error) {
	v := url.Values{}
	v.Set("client_id", APEX_CLIENT_ID)
	v.Set("client_secret", APEX_CLIENT_SECRET)
	dest, err := url.JoinPath(hostname.String(), LOGIN_URL)
	if err != nil {
		return nil, err
	}
	res, err := http.PostForm(dest, v)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %s", string(body))
	}

	var t TokenResponse
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, err
	}

	return &t, nil
}

func (a *Authenticator) pollForResponse(ctx context.Context, t *TokenResponse) error {
	v := url.Values{}
	v.Set("device_code", t.DeviceCode)
	v.Set("client_id", APEX_CLIENT_ID)
	v.Set("client_secret", APEX_CLIENT_SECRET)
	v.Set("grant_type", GRANT_TYPE)

	type response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}

	ticker := time.NewTicker(time.Duration(t.Interval) * time.Second)

	var r response
LOOP:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			dest, err := url.JoinPath(a.hostname.String(), VERIFY_URL)
			if err != nil {
				continue
			}
			res, err := http.PostForm(dest, v)
			if err != nil {
				continue
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				continue
			}
			if res.StatusCode != http.StatusOK {
				continue
			}

			if err := json.Unmarshal(body, &r); err != nil {
				continue
			}

			if r.AccessToken != "" {
				break LOOP
			}
		}
	}
	a.accessToken = r.AccessToken
	a.refreshToken = r.RefreshToken
	return nil
}

func RegisterDevice(hostname *url.URL, publicKey string, accessToken string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"public-key": publicKey,
	})
	if err != nil {
		return "", err
	}

	dest, err := url.JoinPath(hostname.String(), REGISTER_DEVICE)
	if err != nil {
		return "", err
	}

	r, err := http.NewRequest("POST", dest, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	r.Header.Set("authorization", fmt.Sprintf("bearer %s", accessToken))

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusConflict {
		return "", fmt.Errorf("http error: %d %s", res.StatusCode, string(resBody))
	}

	var data DeviceJSON
	if err := json.Unmarshal(resBody, &data); err != nil {
		return "", err
	}

	return data.ID, nil
}

func GetZone(hostname *url.URL, accessToken string) (string, error) {
	dest, err := url.JoinPath(hostname.String(), USER_URL)
	if err != nil {
		return "", err
	}
	r, err := http.NewRequest("GET", dest, nil)
	if err != nil {
		return "", err
	}
	r.Header.Set("authorization", fmt.Sprintf("bearer %s", accessToken))

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http error: %s", string(body))
	}

	type UserJSON struct {
		ID      string   `json:"id"`
		Devices []string `json:"devices"`
		ZoneID  string   `json:"zone-id"`
	}

	var u UserJSON
	if err := json.Unmarshal(body, &u); err != nil {
		return "", err
	}

	return u.ZoneID, nil
}
