package models

import "time"

type LoginStartResponse struct {
	AuthorizationRequestURL string `json:"authorization_request_url"`
}

type LoginEndRequest struct {
	RequestURL string `json:"request_url"`
}

type LoginEndResponse struct {
	Handled      bool   `json:"handled"`
	LoggedIn     bool   `json:"logged_in"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type LogoutResponse struct {
	LogoutURL string `json:"logout_url"`
}

type UserInfoResponse struct {
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	UpdatedAt         int64  `json:"updated_at"`
	FamilyName        string `json:"family_name"`
	Picture           string `json:"picture"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type DeviceStartResponse struct {
	// TODO: Remove this once golang/oauth2 supports device flow
	// and when coreos/go-oidc adds device_authorization_endpoint discovery
	DeviceAuthURL string `json:"device_authorization_endpoint"`
	Issuer        string `json:"issuer"`
	ClientID      string `json:"client_id"`
	// the current time on the server, can be used by a client to get an idea of what the time skew is
	// in relation to the server.
	ServerTime *time.Time `json:"server_time" format:"date-time"`
}

type CheckAuthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
