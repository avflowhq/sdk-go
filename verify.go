// Package sdk provides a Go client for the avflow control plane plus webhook
// signature verification for the events ingest channel.
//
// verify.go is a faithful port of pkg/webhook/sign.go in the (private) avflow
// engine; the fixed test vectors are shared across implementations.
package sdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// SignatureHeader is the canonical header name used by avflow to carry the
// HMAC-SHA256 signature of a webhook / usage-metrics event body. Receivers
// should verify by computing HMAC-SHA256(secret, raw_body) and comparing in
// constant time against this header.
const SignatureHeader = "X-Avflow-Signature"

// SignHMACSHA256 returns the lowercase hex HMAC-SHA256 of body using secret.
// Returns an empty string when secret is empty so callers can simply do
//
//	if sig := sdk.SignHMACSHA256(secret, body); sig != "" {
//	    w.Header().Set(sdk.SignatureHeader, sig)
//	}
//
// without branching on secret presence.
func SignHMACSHA256(secret string, body []byte) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMACSHA256 checks X-Avflow-Signature (sha256=<hex> or bare hex) against
// HMAC-SHA256(secret, body). Empty secret skips verification (dev/test only);
// empty header rejects.
func VerifyHMACSHA256(secret string, body []byte, header string) bool {
	if secret == "" {
		return true
	}
	if header == "" {
		return false
	}
	provided := strings.TrimPrefix(header, "sha256=")
	expected := SignHMACSHA256(secret, body)
	if len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
