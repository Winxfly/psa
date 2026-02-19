package request

type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}
