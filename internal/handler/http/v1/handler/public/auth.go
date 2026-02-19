package public

import (
	"context"
	"net/http"

	"psa/internal/domain"
	"psa/internal/handler/http/v1/handler"
	"psa/internal/handler/http/v1/request"
	"psa/internal/handler/http/v1/response"
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

func (h *AuthHandler) Signin(w http.ResponseWriter, r *http.Request) error {
	log := loggerctx.FromContext(r.Context())

	if r.Method != http.MethodPost {
		return handler.StatusMethodNotAllowed("Method not allowed")
	}

	var req request.SignInRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	tokenPair, err := h.authenticator.Signin(r.Context(), req.Email, req.Password)
	if err != nil {
		log.Warn("auth.signin.failed", "reason", "invalid_credentials")
		return handler.StatusUnauthorized("Invalid credentials")
	}

	log.Info("auth.signin.success")

	resp := response.TokenPairResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}

	handler.RespondJSON(w, http.StatusOK, resp)

	return nil
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) error {
	log := loggerctx.FromContext(r.Context())

	if r.Method != http.MethodPost {
		return handler.StatusMethodNotAllowed("Method not allowed")
	}

	var req request.RefreshTokenRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	tokenPair, err := h.authenticator.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		log.Warn("auth.token.refresh.failed", "reason", "invalid_token")
		return handler.StatusUnauthorized("Invalid refresh token")
	}

	resp := response.TokenPairResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	log := loggerctx.FromContext(r.Context())

	if r.Method != http.MethodPost {
		return handler.StatusMethodNotAllowed("Method not allowed")
	}

	var req request.LogoutRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	if err := h.authenticator.Logout(r.Context(), req.RefreshToken); err != nil {
		log.Warn("auth.logout.failed", "error", err)
	}

	log.Info("auth.logout.success")

	handler.RespondJSON(w, http.StatusOK, response.LogoutResponse{
		Message: "Successfully logged out",
	})

	return nil
}
