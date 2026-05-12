package zerodha

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const nonceTTL = 10 * time.Minute

// NewNonce generates a stateless signed nonce encoding userID and a timestamp.
// Format (before base64): "<userID>:<unix_ts>.<hex_hmac>"
// The nonce is safe to include in a URL query param.
func NewNonce(userID, serverSecret string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	payload := userID + ":" + ts
	sig := sign(payload, serverSecret)
	raw := payload + "." + sig
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// VerifyNonce decodes and verifies a nonce, returning the embedded userID.
// Returns an error if the signature is invalid or the nonce has expired.
func VerifyNonce(nonce, serverSecret string) (userID string, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}

	parts := strings.SplitN(string(raw), ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed nonce")
	}
	payload, sig := parts[0], parts[1]

	expected := sign(payload, serverSecret)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", fmt.Errorf("invalid nonce signature")
	}

	idx := strings.LastIndex(payload, ":")
	if idx < 0 {
		return "", fmt.Errorf("malformed nonce payload")
	}
	tsStr := payload[idx+1:]
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("malformed nonce timestamp")
	}
	if time.Since(time.Unix(ts, 0)) > nonceTTL {
		return "", fmt.Errorf("nonce expired")
	}

	return payload[:idx], nil
}

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
