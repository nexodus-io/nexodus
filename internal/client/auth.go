package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	agent "github.com/redhat-et/go-oidc-agent"
	log "github.com/sirupsen/logrus"
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

func NewTokenAuthenticator(token string) Authenticator {
	return &TokenAuthenticator{
		accessToken: token,
	}
}

func (a *TokenAuthenticator) Token() (string, error) {
	return a.accessToken, nil
}

type DeviceFlowAuthenticator struct {
	clientID      string
	deviceURL     string
	tokenURL      string
	accessToken   string
	refreshToken  string
	tokenExpiry   time.Time
	refreshExpiry time.Time
}

func NewDeviceFlowAuthenticator(ctx context.Context, hostname url.URL) (*DeviceFlowAuthenticator, error) {
	a := &DeviceFlowAuthenticator{}

	if err := a.startLogin(hostname); err != nil {
		return nil, err
	}
	requestTime := time.Now()
	token, err := a.getToken()
	if err != nil {
		return nil, err
	}

	fmt.Println("Your device must be registered with Apex.")
	fmt.Printf("Your one-time code is: %s\n", token.UserCode)
	fmt.Println("Please open the following URL in your browser to sign in:")
	fmt.Printf("%s\n", token.VerificationURIComplete)

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

func (a *DeviceFlowAuthenticator) startLogin(hostname url.URL) error {
	dest := hostname
	dest.Path = "/login/start"
	res, err := http.Post(dest.String(), "application/json", nil)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("request %s failed with %d", dest.String(), res.StatusCode)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var resp agent.DeviceStartReponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return err
	}

	a.clientID = resp.ClientID
	a.deviceURL = resp.DeviceAuthURL
	a.tokenURL = resp.TokenEndpoint

	return nil
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
	v.Set("client_id", a.clientID)
	v.Set("grant_type", "refresh_token")
	v.Set("refresh_token", a.refreshToken)
	requestTime := time.Now()
	res, err := http.PostForm(a.tokenURL, v)
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

func (a *DeviceFlowAuthenticator) getToken() (*TokenResponse, error) {
	v := url.Values{}
	v.Set("client_id", a.clientID)
	v.Set("scope", "openid profile email")
	res, err := http.PostForm(a.deviceURL, v)
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
	Error            string `json:"error"`
}

const (
	errAuthorizationPending = "authorization_pending"
	errSlowDown             = "slow_down"
	errAccessDenied         = "access_denied"
	errExpiredToken         = "expired_token"
)

func (a *DeviceFlowAuthenticator) pollForResponse(ctx context.Context, t *TokenResponse, requestTime time.Time) error {
	v := url.Values{}
	v.Set("device_code", t.DeviceCode)
	v.Set("client_id", a.clientID)
	v.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	interval := t.Interval
	if interval == 0 {
		// Pick a reasonable default if none is set
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	var r tokenReponse
LOOP:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			res, err := http.PostForm(a.tokenURL, v)
			if err != nil {
				continue
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				continue
			}
			if res.StatusCode != http.StatusOK {
				if err := json.Unmarshal(body, &r); err != nil {
					continue
				}
				if r.Error == errSlowDown {
					interval += 5
					ticker.Reset(time.Duration(interval) * time.Second)
					continue
				}
				if r.Error == errAccessDenied || r.Error == errExpiredToken {
					return fmt.Errorf("failed to get token: %s", r.Error)
				}
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
