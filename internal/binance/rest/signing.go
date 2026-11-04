package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func Sign(query, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}
