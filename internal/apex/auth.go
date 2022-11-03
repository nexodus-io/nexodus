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

	log "github.com/sirupsen/logrus"
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

type Authenticator interface {
	Token() (string, error)
}

type TokenAuthenticator struct {
	accessToken string
}

func (a *TokenAuthenticator) Token() (string, error) {
	return a.accessToken, nil
}

type DeviceFlowAuthenticator struct {
	hostname      *url.URL
	accessToken   string
	refreshToken  string
	tokenExpiry   time.Time
	refreshExpiry time.Time
}

func NewDeviceFlowAuthenticator(ctx context.Context, hostname *url.URL) (*DeviceFlowAuthenticator, error) {
	a := &DeviceFlowAuthenticator{
		hostname: hostname,
	}
	requestTime := time.Now()
	token, err := getToken(a.hostname)
	if err != nil {
		return nil, err
	}

	fmt.Println("Your device must be registered with Apex Controller.")
	fmt.Printf("Your one-time code is: %s\n", token.UserCode)
	fmt.Println("Please open the following URL in your browser and enter your one-time code:")
	dest, err := url.JoinPath(a.hostname.String(), VERIFICATION_URI)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%s\n", dest)

	c := make(chan error, 1)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(token.ExpiresIn)*time.Second)
	defer cancel()
	go func() {
		c <- a.pollForResponse(ctx, token, requestTime)
	}()

	err = <-c
	if err != nil {
		return nil, err
	}
	fmt.Println("Authentication succeeded.")
	return a, nil
}

func (a *DeviceFlowAuthenticator) Token() (string, error) {
	if time.Now().After(a.tokenExpiry) {
		log.Debugf("Access token has expired. Requesting a new one")
		if time.Now().After(a.refreshExpiry) {
			return "", fmt.Errorf("refresh token has expired")
		}
		if err := a.refreshTokens(); err != nil {
			return "", err
		}
	}
	if a.accessToken != "" {
		return a.accessToken, nil
	}
	return "", fmt.Errorf("not authenticated")
}

func (a *DeviceFlowAuthenticator) refreshTokens() error {
	v := url.Values{}
	v.Set("client_id", APEX_CLIENT_ID)
	v.Set("client_secret", APEX_CLIENT_SECRET)
	v.Set("grant_type", "refresh_token")
	v.Set("refresh_token", a.refreshToken)
	dest, err := url.JoinPath(a.hostname.String(), VERIFY_URL)
	if err != nil {
		return err
	}
	requestTime := time.Now()
	res, err := http.PostForm(dest, v)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return err
	}
	var r tokenReponse
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	a.accessToken = r.AccessToken
	a.tokenExpiry = requestTime.Add(time.Duration(r.ExpiresIn) * time.Second)
	a.refreshToken = r.RefreshToken
	a.refreshExpiry = requestTime.Add(time.Duration(r.RefreshExpiresIn) * time.Second)
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

type tokenReponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

func (a *DeviceFlowAuthenticator) pollForResponse(ctx context.Context, t *TokenResponse, requestTime time.Time) error {
	v := url.Values{}
	v.Set("device_code", t.DeviceCode)
	v.Set("client_id", APEX_CLIENT_ID)
	v.Set("client_secret", APEX_CLIENT_SECRET)
	v.Set("grant_type", GRANT_TYPE)

	ticker := time.NewTicker(time.Duration(t.Interval) * time.Second)
	var r tokenReponse
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
	a.tokenExpiry = requestTime.Add(time.Duration(r.ExpiresIn) * time.Second)
	a.refreshToken = r.RefreshToken
	a.refreshExpiry = requestTime.Add(time.Duration(r.RefreshExpiresIn) * time.Second)
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
