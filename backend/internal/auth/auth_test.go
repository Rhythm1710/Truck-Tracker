package auth

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "this-is-a-test-secret-that-is-long-enough-32+"

func TestIssueVerify_Roundtrip(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	tok, err := Issue(testSecret, Claims{UserID: "u1", Phone: "+91...", ExpiresAt: exp})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if tok == "" {
		t.Fatal("empty token")
	}

	c, err := Verify(testSecret, tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if c.UserID != "u1" {
		t.Errorf("UserID got %q want u1", c.UserID)
	}
	if c.Phone != "+91..." {
		t.Errorf("Phone got %q", c.Phone)
	}
	if c.ExpiresAt != exp {
		t.Errorf("Exp got %d want %d", c.ExpiresAt, exp)
	}
}

func TestVerify_RejectsTamperedSignature(t *testing.T) {
	tok, _ := Issue(testSecret, Claims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	// Flip the last char of the signature.
	parts := strings.Split(tok, ".")
	if len(parts) != 2 {
		t.Fatalf("unexpected token shape %q", tok)
	}
	sig := []byte(parts[1])
	sig[len(sig)-1] ^= 1
	tampered := parts[0] + "." + string(sig)
	if _, err := Verify(testSecret, tampered); err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerify_RejectsTamperedBody(t *testing.T) {
	tok, _ := Issue(testSecret, Claims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	parts := strings.Split(tok, ".")
	body := []byte(parts[0])
	body[0] ^= 1
	if _, err := Verify(testSecret, string(body)+"."+parts[1]); err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerify_RejectsExpired(t *testing.T) {
	tok, _ := Issue(testSecret, Claims{UserID: "u1", ExpiresAt: time.Now().Add(-time.Second).Unix()})
	if _, err := Verify(testSecret, tok); err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerify_RejectsMalformed(t *testing.T) {
	cases := []string{"", "not-a-token", "only-one-part", ".", "a.b.c"}
	for _, c := range cases {
		if _, err := Verify(testSecret, c); err != ErrTokenInvalid {
			t.Errorf("Verify(%q) = %v, want ErrTokenInvalid", c, err)
		}
	}
}

func TestIssue_RequiresExp(t *testing.T) {
	if _, err := Issue(testSecret, Claims{UserID: "u1"}); err == nil {
		t.Fatal("expected error for missing exp")
	}
}

func TestVerify_RejectsCrossSecret(t *testing.T) {
	tok, _ := Issue(testSecret, Claims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	if _, err := Verify("different-secret-also-long-enough-32+chars", tok); err != ErrTokenInvalid {
		t.Fatalf("cross-secret verify should fail with ErrTokenInvalid, got %v", err)
	}
}
