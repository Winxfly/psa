package public

import (
	"context"
	"log/slog"
	"net/http"
	"psa/internal/controller/http/v1/handler"
	"psa/internal/controller/http/v1/request"
	"psa/internal/controller/http/v1/response"
	"psa/internal/entity"
)

type Authenticator interface {
	Signin(ctx context.Context, email, password string) (*entity.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*entity.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}

type AuthHandler struct {
	log           *slog.Logger
	authenticator Authenticator
}

func NewAuthHandler(log *slog.Logger, authenticator Authenticator) *AuthHandler {
	return &AuthHandler{
		log:           log,
		authenticator: authenticator,
	}
}

func (h *AuthHandler) Signin(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return handler.StatusMethodNotAllowed("Method not allowed")
	}

	var req request.SignInRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	tokenPair, err := h.authenticator.Signin(r.Context(), req.Email, req.Password)
	if err != nil {
		return handler.StatusUnauthorized("Invalid credentials")
	}

	resp := response.TokenPairResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
	}

	handler.RespondJSON(w, http.StatusOK, resp)
	return nil
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return handler.StatusMethodNotAllowed("Method not allowed")
	}

	var req request.RefreshTokenRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	tokenPair, err := h.authenticator.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
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
	if r.Method != http.MethodPost {
		return handler.StatusMethodNotAllowed("Method not allowed")
	}

	var req request.LogoutRequest
	if err := handler.DecodeJSON(r, &req); err != nil {
		return err
	}

	if err := h.authenticator.Logout(r.Context(), req.RefreshToken); err != nil {
		// TODO: не возвращаем ошибку тк должен завершаться успешно? переделать
	}

	handler.RespondJSON(w, http.StatusOK, response.LogoutResponse{
		Message: "Successfully logged out",
	})

	return nil
}
