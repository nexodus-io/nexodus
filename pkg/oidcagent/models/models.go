package models

type LoginStartResponse struct {
	AuthorizationRequestURL string `json:"authorization_request_url"`
}

type LoginEndRequest struct {
	RequestURL string `json:"request_url"`
}

type LoginEndResponse struct {
	Handled  bool `json:"handled"`
	LoggedIn bool `json:"logged_in"`
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

type DeviceStartResponse struct {
	// TODO: Remove this once golang/oauth2 supports device flow
	// and when coreos/go-oidc adds device_authorization_endpoint discovery
	DeviceAuthURL string `json:"device_authorization_endpoint"`
	Issuer        string `json:"issuer"`
	ClientID      string `json:"client_id"`
}

type CheckAuthResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
