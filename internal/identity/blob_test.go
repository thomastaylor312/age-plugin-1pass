package identity

import (
	"strings"
	"testing"
)

func TestBlobRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		in   Blob
	}{
		{
			name: "x25519 default",
			in: Blob{
				Account: "my.1password.com",
				Ref:     "op://Personal/age-test/password",
				Type:    KeyTypeX25519,
			},
		},
		{
			name: "hybrid pq",
			in: Blob{
				Account: "ABCDEF1234567890",
				Ref:     "op://Work/age-pq/password",
				Type:    KeyTypeHybrid,
			},
		},
		{
			name: "type defaults to x25519 when empty",
			in: Blob{
				Account: "acme",
				Ref:     "op://Shared/key/password",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.in.Encode()
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := Decode(raw)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			want := tc.in
			if want.Type == "" {
				want.Type = KeyTypeX25519
			}
			if got != want {
				t.Errorf("round trip mismatch:\n got  %+v\n want %+v", got, want)
			}
		})
	}
}

func TestEncodeDeterministic(t *testing.T) {
	b := Blob{Account: "a", Ref: "op://v/i/password", Type: KeyTypeX25519}
	first, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		again, err := b.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if string(again) != string(first) {
			t.Fatalf("Encode is non-deterministic:\n first %q\n again %q", first, again)
		}
	}
}

func TestDecodeIgnoresUnknownKeys(t *testing.T) {
	raw := []byte("v=1\naccount=a\nref=op://v/i/password\ntype=x25519\nfuture=whatever\n")
	got, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Account != "a" || got.Ref != "op://v/i/password" || got.Type != KeyTypeX25519 {
		t.Errorf("unexpected blob: %+v", got)
	}
}

func TestDecodeErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"missing version", "account=a\nref=op://v/i/p\n", "missing 'v='"},
		{"wrong version", "v=2\naccount=a\nref=op://v/i/p\n", "unsupported version"},
		{"missing equals", "v=1\naccount a\nref=op://v/i/p\n", "missing '=' separator"},
		{"missing account", "v=1\nref=op://v/i/p\n", "account is required"},
		{"missing ref", "v=1\naccount=a\n", "ref is required"},
		{"bad ref scheme", "v=1\naccount=a\nref=/not/op\n", "must start with op://"},
		{"unknown type", "v=1\naccount=a\nref=op://v/i/p\ntype=weird\n", "unknown key type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.raw))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestEncodeRejectsNewlineInFields(t *testing.T) {
	_, err := Blob{Account: "a\nb", Ref: "op://v/i/p"}.Encode()
	if err == nil {
		t.Fatal("expected error for newline in account")
	}
}
