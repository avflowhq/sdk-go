package sdk

import "testing"

const (
	testSecret = "avflow-test-secret"
	// Fixed vector shared with the TS implementation (and computable with
	// openssl): HMAC-SHA256 over this exact body.
	testBody   = `{"schema":"avflow.events.v1","event":"job.started"}`
	testVector = "0ea5dca25cbd2ab146719b0488d499d480a75bd335c82a6c725d4614c375db42"
)

func TestSignHMACSHA256MatchesFixedVector(t *testing.T) {
	if got := SignHMACSHA256(testSecret, []byte(testBody)); got != testVector {
		t.Errorf("signature = %q, want %q", got, testVector)
	}
}

func TestSignHMACSHA256EmptySecret(t *testing.T) {
	if got := SignHMACSHA256("", []byte(testBody)); got != "" {
		t.Errorf("empty secret signature = %q, want empty", got)
	}
}

func TestVerifyHMACSHA256(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		body   string
		header string
		want   bool
	}{
		{"bare hex", testSecret, testBody, testVector, true},
		{"sha256= prefix", testSecret, testBody, "sha256=" + testVector, true},
		{"tampered body", testSecret, testBody + " ", testVector, false},
		{"wrong secret", "other", testBody, testVector, false},
		{"empty header", testSecret, testBody, "", false},
		{"empty secret skips", "", testBody, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := VerifyHMACSHA256(tc.secret, []byte(tc.body), tc.header); got != tc.want {
				t.Errorf("VerifyHMACSHA256(%q, %q, %q) = %v, want %v",
					tc.secret, tc.body, tc.header, got, tc.want)
			}
		})
	}
}
