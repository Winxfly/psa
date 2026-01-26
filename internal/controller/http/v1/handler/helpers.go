package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

type HTTPError struct {
	Status int
	Msg    string
}

func (e *HTTPError) Error() string {
	return e.Msg
}

func NewHTTPError(status int, msg string) *HTTPError {
	return &HTTPError{Status: status, Msg: msg}
}

func StatusBadRequest(msg string) *HTTPError   { return NewHTTPError(http.StatusBadRequest, msg) }
func StatusUnauthorized(msg string) *HTTPError { return NewHTTPError(http.StatusUnauthorized, msg) }
func StatusForbidden(msg string) *HTTPError    { return NewHTTPError(http.StatusForbidden, msg) }
func StatusNotFound(msg string) *HTTPError     { return NewHTTPError(http.StatusNotFound, msg) }
func StatusConflict(msg string) *HTTPError     { return NewHTTPError(http.StatusConflict, msg) }
func StatusMethodNotAllowed(msg string) *HTTPError {
	return NewHTTPError(http.StatusMethodNotAllowed, msg)
}

func StatusInternalServerError(msg string) *HTTPError {
	return NewHTTPError(http.StatusInternalServerError, msg)
}

func Handle(f func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			handleError(w, r, err)
		}
	}
}

func handleError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	msg := http.StatusText(status)

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.Status
		msg = httpErr.Msg
	}

	slog.ErrorContext(r.Context(), "handler error",
		"error", err,
		"status", status,
		"method", r.Method,
		"path", r.URL.Path,
		"ip", clientIP(r),
	)

	sendJSON(w, status, map[string]string{"error": msg})
}

func DecodeJSON(r *http.Request, v any) error {
	if r.Body == http.NoBody {
		return StatusBadRequest("request body is required")
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		var syntaxError *json.SyntaxError
		var typeErr *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			return StatusBadRequest(fmt.Sprintf("invalid JSON syntax at offset %d", syntaxError.Offset))
		case errors.As(err, &typeErr):
			return StatusBadRequest(fmt.Sprintf("wrong type for field %q", typeErr.Field))
		case errors.Is(err, io.EOF):
			return StatusBadRequest("request body is required")
		default:
			return StatusBadRequest("invalid JSON: " + err.Error())
		}
	}

	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return StatusBadRequest("unexpected JSON data after body")
	}

	return nil
}

func RespondJSON(w http.ResponseWriter, status int, v any) {
	sendJSON(w, status, v)
}

func sendJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	if err := enc.Encode(v); err != nil {
		slog.Debug("failed to encode JSON response", "error", err)
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func PathUUID(r *http.Request, param string) (uuid.UUID, error) {
	value := r.PathValue(param)
	if value == "" {
		return uuid.Nil, StatusBadRequest(fmt.Sprintf("missing path parameter: %s", param))
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, StatusBadRequest(fmt.Sprintf("invalid uuid for parameter: %s", param))
	}

	return id, nil
}
