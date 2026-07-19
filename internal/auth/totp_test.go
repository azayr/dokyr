package auth

import (
	"testing"
	"time"
)

func TestVerifyTOTPAcceptsRFC6238SHA1Code(t *testing.T) {
	// RFC 6238 shared secret "12345678901234567890", encoded as Base32.
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	now := time.Unix(59, 0)
	if !VerifyTOTP(secret, "287082", now) {
		t.Fatal("expected six-digit RFC 6238 code to be accepted")
	}
	if VerifyTOTP(secret, "287083", now) {
		t.Fatal("expected incorrect code to be rejected")
	}
}

func TestTOTPURIContainsAccountAndIssuer(t *testing.T) {
	uri := TOTPURI("ABC123", "Selfhost", "owner@example.test")
	if uri != "otpauth://totp/Selfhost:owner@example.test?algorithm=SHA1&digits=6&issuer=Selfhost&period=30&secret=ABC123" {
		t.Fatalf("unexpected URI: %s", uri)
	}
}
