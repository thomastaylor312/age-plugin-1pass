package identity

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"filippo.io/age"
)

// fakeResolver is a test double for SecretResolver that returns a canned
// string and counts calls so we can assert lazy/one-shot behavior.
type fakeResolver struct {
	value string
	err   error
	calls atomic.Int32
}

func (f *fakeResolver) Resolve(_ context.Context, _ string) (string, error) {
	f.calls.Add(1)
	return f.value, f.err
}

// encryptToRecipient is a tiny helper that encrypts "hello" for the given
// recipient and returns the resulting age file bytes. We use this to build
// real ciphertexts for round-trip Unwrap tests without touching 1Password.
func encryptToRecipient(t *testing.T, r age.Recipient) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, r)
	if err != nil {
		t.Fatalf("age.Encrypt: %v", err)
	}
	if _, err := io.WriteString(w, "hello"); err != nil {
		t.Fatalf("write plaintext: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close encrypt writer: %v", err)
	}
	return buf.Bytes()
}

func decryptAll(t *testing.T, ciphertext []byte, id age.Identity) string {
	t.Helper()
	r, err := age.Decrypt(bytes.NewReader(ciphertext), id)
	if err != nil {
		t.Fatalf("age.Decrypt: %v", err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read plaintext: %v", err)
	}
	return string(b)
}

func TestOnePassIdentityX25519RoundTrip(t *testing.T) {
	real, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}
	ciphertext := encryptToRecipient(t, real.Recipient())

	resolver := &fakeResolver{value: real.String()}
	blob := Blob{Account: "acct", Ref: "op://v/i/password", Type: KeyTypeX25519}
	id, err := New(context.Background(), blob, resolver)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := decryptAll(t, ciphertext, id); got != "hello" {
		t.Errorf("plaintext = %q, want %q", got, "hello")
	}

	// A second decrypt must not cause a second resolver call — the
	// fetched secret should be cached inside the OnePassIdentity.
	if got := decryptAll(t, ciphertext, id); got != "hello" {
		t.Errorf("second plaintext = %q, want %q", got, "hello")
	}
	if calls := resolver.calls.Load(); calls != 1 {
		t.Errorf("resolver called %d times, want 1 (should be cached)", calls)
	}
}

func TestOnePassIdentityHybridRoundTrip(t *testing.T) {
	real, err := age.GenerateHybridIdentity()
	if err != nil {
		t.Fatalf("GenerateHybridIdentity: %v", err)
	}
	ciphertext := encryptToRecipient(t, real.Recipient())

	resolver := &fakeResolver{value: real.String()}
	blob := Blob{Account: "acct", Ref: "op://v/i/password", Type: KeyTypeHybrid}
	id, err := New(context.Background(), blob, resolver)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := decryptAll(t, ciphertext, id); got != "hello" {
		t.Errorf("plaintext = %q, want %q", got, "hello")
	}
}

func TestOnePassIdentityResolverError(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("boom")}
	blob := Blob{Account: "a", Ref: "op://v/i/password", Type: KeyTypeX25519}
	id, err := New(context.Background(), blob, resolver)
	if err != nil {
		t.Fatal(err)
	}
	_, err = id.Unwrap(nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped resolver error, got %v", err)
	}
}

func TestOnePassIdentityParseError(t *testing.T) {
	resolver := &fakeResolver{value: "not-a-valid-age-secret-key"}
	blob := Blob{Account: "a", Ref: "op://v/i/password", Type: KeyTypeX25519}
	id, err := New(context.Background(), blob, resolver)
	if err != nil {
		t.Fatal(err)
	}
	_, err = id.Unwrap(nil)
	if err == nil || !strings.Contains(err.Error(), "parse X25519 identity") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestNewRejectsNilResolver(t *testing.T) {
	if _, err := New(context.Background(), Blob{}, nil); err == nil {
		t.Fatal("expected error for nil resolver")
	}
}
