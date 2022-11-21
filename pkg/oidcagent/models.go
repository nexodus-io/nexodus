package agent

type LoginStartReponse struct {
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
