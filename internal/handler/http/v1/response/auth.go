package response

type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}
