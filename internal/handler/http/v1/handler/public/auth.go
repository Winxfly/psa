package public

import (
	"context"
	"net/http"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/pkg/logger/loggerctx"
)

type Authenticator interface {
	Signin(ctx context.Context, email, password string) (*domain.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*domain.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}

type AuthHandler struct {
	authenticator Authenticator
}

func NewAuthHandler(authenticator Authenticator) *AuthHandler {
	return &AuthHandler{
		authenticator: authenticator,
	}
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Signin(w http.ResponseWriter, r *http.Request) error {
	log := loggerctx.FromContext(r.Context())

	var req signInRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	tokenPair, err := h.authenticator.Signin(r.Context(), req.Email, req.Password)
	if err != nil {
		log.Warn("auth_signin_failed", "reason", "invalid_credentials")
		return handler.StatusUnauthorized("Invalid credentials")
	}

	log.Info("auth_signin_success")

	resp := tokenPairResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}

	handler.RespondJSON(w, http.StatusOK, resp)

	return nil
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) error {
	log := loggerctx.FromContext(r.Context())

	var req refreshTokenRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	tokenPair, err := h.authenticator.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		log.Warn("auth_token_refresh_failed", "reason", "invalid_token")
		return handler.StatusUnauthorized("Invalid refresh token")
	}

	resp := tokenPairResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutResponse struct {
	Message string `json:"message"`
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	log := loggerctx.FromContext(r.Context())

	var req logoutRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	if err := h.authenticator.Logout(r.Context(), req.RefreshToken); err != nil {
		log.Warn("auth_logout_failed", "error", err)
		return handler.StatusInternalServerError("Failed to logout")
	}

	log.Info("auth_logout_success")

	handler.RespondJSON(w, http.StatusOK, logoutResponse{
		Message: "Successfully logged out",
	})

	return nil
}
