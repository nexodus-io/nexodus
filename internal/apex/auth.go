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
	LOGIN_URL          = "http://%s/auth/realms/controller/protocol/openid-connect/auth/device"
	VERIFICATION_URI   = "http://%s/auth/realms/controller/device"
	VERIFY_URL         = "http://%s/auth/realms/controller/protocol/openid-connect/token"
	GRANT_TYPE         = "urn:ietf:params:oauth:grant-type:device_code"
	REGISTER_DEVICE    = "http://%s/api/devices"
	USER_URL           = "http://%s/api/users/me"
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
	hostname     string
	accessToken  string
	refreshToken string
}

func NewAuthenticator(hostname string) Authenticator {
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
	fmt.Printf("%s\n", fmt.Sprintf(VERIFICATION_URI, a.hostname))

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

func getToken(hostname string) (*TokenResponse, error) {
	v := url.Values{}
	v.Set("client_id", APEX_CLIENT_ID)
	v.Set("client_secret", APEX_CLIENT_SECRET)
	res, err := http.PostForm(fmt.Sprintf(LOGIN_URL, hostname), v)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
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
			res, err := http.PostForm(fmt.Sprintf(VERIFY_URL, a.hostname), v)
			if err != nil {
				continue
			}
			body, err := io.ReadAll(res.Body)
			if err != nil {
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

func RegisterDevice(hostname string, publicKey string, accessToken string) error {
	body, err := json.Marshal(map[string]string{
		"public-key": publicKey,
	})
	if err != nil {
		return err
	}

	r, err := http.NewRequest("POST", fmt.Sprintf(REGISTER_DEVICE, hostname), bytes.NewReader(body))
	if err != nil {
		return err
	}
	r.Header.Set("authorization", fmt.Sprintf("bearer %s", accessToken))

	if _, err := http.DefaultClient.Do(r); err != nil {
		return err
	}
	return nil
}

func GetZone(hostname string, accessToken string) (string, error) {
	r, err := http.NewRequest("GET", fmt.Sprintf(USER_URL, hostname), nil)
	if err != nil {
		return "", err
	}
	r.Header.Set("authorization", fmt.Sprintf("bearer %s", accessToken))

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	log.Debugf("%+v", string(body))
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
