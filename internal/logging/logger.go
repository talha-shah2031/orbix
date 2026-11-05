package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func New(path string, level slog.Level) (*slog.Logger, io.Closer, error) {
	if path == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return nil, nil, fmt.Errorf("resolve cache directory: %w", err)
		}
		path = filepath.Join(dir, "orbix", "orbix.log")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	handler := slog.NewJSONHandler(file, &slog.HandlerOptions{Level: level})
	return slog.New(NewRedactingHandler(handler)), file, nil
}
