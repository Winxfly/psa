package slogx

import (
	"log/slog"
)

// Err returns a slog.Attr with key "error" containing the error message.
// If err is nil, returns an empty string value.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	return slog.Attr{
		Key:   "error",
		Value: slog.StringValue(err.Error()),
	}
}
