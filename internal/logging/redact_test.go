package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedact(t *testing.T) {
	input := `url?signature=abcdef&x=1 API_KEY: supersecret {"apiKey":"json-key","secret":"json-secret"}`
	result := Redact(input)
	require.NotContains(t, result, "abcdef")
	require.NotContains(t, result, "supersecret")
	require.NotContains(t, result, "json-key")
	require.NotContains(t, result, "json-secret")
}

func TestRedactingHandler(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewJSONHandler(&buffer, nil)))
	logger.InfoContext(context.Background(), "request signature=abc", "api_key", "xyz")
	require.NotContains(t, buffer.String(), "abc")
	require.NotContains(t, buffer.String(), "xyz")
	require.Contains(t, buffer.String(), masked)
}
