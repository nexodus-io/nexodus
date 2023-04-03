package client

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nexodus-io/nexodus/pkg/oidcagent/models"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

type deviceFlowResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

func newDeviceFlowToken(ctx context.Context, deviceEndpoint, tokenEndpoint, clientID string, authcb func(string)) (*oauth2.Token, interface{}, error) {
	requestTime := time.Now()
	d, err := startDeviceFlow(deviceEndpoint, clientID)
	if err != nil {
		return nil, nil, err
	}

	msg := fmt.Sprintf("Your device must be registered with Nexodus.\n"+
		"Your one-time code is: %s\n"+
		"Please open the following URL in your browser to sign in:\n%s\n",
		d.UserCode, d.VerificationURIComplete)
	fmt.Print(msg)
	if authcb != nil {
		authcb(msg)
	}

	var token *oauth2.Token
	var idToken interface{}
	c := make(chan error, 1)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(d.ExpiresIn)*time.Second)
	defer cancel()
	go func() {
		token, idToken, err = pollForResponse(ctx, clientID, tokenEndpoint, d, requestTime)
		c <- err
	}()

	err = <-c
	if err != nil {
		return nil, nil, err
	}

	fmt.Println("Authentication succeeded.")

	return token, idToken, nil
}

func startLogin(client *http.Client, hostname url.URL) (*models.DeviceStartReponse, error) {
	dest := hostname
	dest.Path = "/device/login/start"
	res, err := client.Post(dest.String(), "application/json", nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request %s failed with %d", dest.String(), res.StatusCode)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resp models.DeviceStartReponse
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func startDeviceFlow(deviceEndpoint string, clientID string) (*deviceFlowResponse, error) {
	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("scope", "openid profile email offline_access read:organizations write:organizations read:users write:users read:devices write:devices")
	// #nosec -- G107: Potential HTTP request made with variable url (gosec)
	res, err := http.PostForm(deviceEndpoint, v)
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

	var t deviceFlowResponse
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, err
	}

	return &t, nil
}

const (
	errAuthorizationPending = "authorization_pending"
	errSlowDown             = "slow_down"
	errAccessDenied         = "access_denied"
	errExpiredToken         = "expired_token"
)

func pollForResponse(ctx context.Context, clientID string, tokenURL string, t *deviceFlowResponse, requestTime time.Time) (*oauth2.Token, interface{}, error) {
	v := url.Values{}
	v.Set("device_code", t.DeviceCode)
	v.Set("client_id", clientID)
	v.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	interval := t.Interval
	if interval == 0 {
		// Pick a reasonable default if none is set
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	var token oauth2.Token
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-ticker.C:
			requestTime := time.Now()
			// #nosec -- G107: Potential HTTP request made with variable url (gosec)
			res, err := http.PostForm(tokenURL, v)
			if err != nil {
				// possible transient connection error, continue retrying
				continue
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				// possible transient connection error, continue retrying
				continue
			}
			if res.StatusCode != http.StatusOK {
				type errorResponse struct {
					Error string `json:"error"`
				}
				var r errorResponse
				if err := json.Unmarshal(body, &r); err != nil {
					return nil, "", err
				}
				if r.Error != "" {
					if r.Error == errSlowDown {
						// adjust interval and continue retrying
						interval += 5
						ticker.Reset(time.Duration(interval) * time.Second)
						continue
					} else if r.Error == errAccessDenied || r.Error == errExpiredToken {
						return nil, nil, fmt.Errorf("failed to get token: %s", r.Error)
					}
					// error was either authorization_pending or something else
					// continue to poll for a token
					continue
				}
			}
			// This will only give us AccessToken, RefreshToken and TokenType
			if err := json.Unmarshal(body, &token); err != nil {
				return nil, "", err
			}
			// We need the OIDC id_token from here, but also the expires_in to compute the expiry time
			var tokenRaw map[string]interface{}
			if err = json.Unmarshal(body, &tokenRaw); err != nil {
				return nil, "", err
			}

			expiresRaw, ok := tokenRaw["expires_in"]
			if !ok {
				return nil, "", fmt.Errorf("expires_in is not contained in the access token")
			}

			expires, ok := expiresRaw.(float64)
			if !ok {
				return nil, "", fmt.Errorf("cannot cast expires_in to float64")
			}

			if expires == 0 {
				return nil, "", fmt.Errorf("expires_in should not be 0")
			}
			token.Expiry = requestTime.Add(time.Duration(expires) * time.Second)
			idToken := tokenRaw["id_token"]
			return &token, idToken, nil
		}
	}
}
