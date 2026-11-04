package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type APIError struct {
	StatusCode int
	Code       int
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("Binance API error %d (HTTP %d): %s", e.Code, e.StatusCode, redact(e.Message))
	}
	return fmt.Sprintf("Binance API HTTP %d: %s", e.StatusCode, redact(e.Message))
}

func redact(value string) string {
	return sensitiveValue.ReplaceAllString(value, "$1=[REDACTED]")
}

var sensitiveValue = regexp.MustCompile(`(?i)(signature|api[-_]?key|secret)=([^&\s,;]+)`)

func (e *APIError) IsRateLimit() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode == http.StatusTeapot
}
func (e *APIError) IsTimestamp() bool { return e.Code == -1021 }
func (e *APIError) IsAuth() bool {
	return e.StatusCode == http.StatusUnauthorized || e.Code == -2014 || e.Code == -2015
}
func (e *APIError) IsGeoBlocked() bool {
	return e.StatusCode == http.StatusUnavailableForLegalReasons || strings.Contains(strings.ToLower(e.Message), "restricted location") || strings.Contains(strings.ToLower(e.Message), "not available in your region")
}

type errorPayload struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func parseAPIError(status int, retryAfter string, body []byte) *APIError {
	payload := errorPayload{}
	_ = json.Unmarshal(body, &payload)
	message := payload.Msg
	if message == "" {
		message = http.StatusText(status)
	}
	delay := parseRetryAfter(retryAfter)
	if status == http.StatusTeapot && delay <= 0 {
		delay = 2 * time.Minute
	}
	return &APIError{StatusCode: status, Code: payload.Code, Message: message, RetryAfter: delay}
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if until, err := http.ParseTime(value); err == nil {
		if delay := time.Until(until); delay > 0 {
			return delay
		}
	}
	return 0
}

func GeoBlockMessage() string {
	return "Binance denied this request from the current location (HTTP 451). For public market data, configure https://data-api.binance.vision where available."
}
