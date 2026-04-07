// Package identity handles the self-contained payload that age-plugin-1pass
// embeds inside the bech32 AGE-PLUGIN-1PASS-1... identity string.
//
// The payload is a small newline-delimited key=value block. It is intentionally
// human-readable so that operators can decode an identity file (via
// `plugin.ParseIdentity`) and inspect what 1Password account and secret
// reference it points at without running the plugin.
package identity

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// KeyType identifies which age identity format the stored secret uses.
type KeyType string

const (
	// KeyTypeX25519 is the standard age X25519 identity (AGE-SECRET-KEY-1...).
	KeyTypeX25519 KeyType = "x25519"
	// KeyTypeHybrid is the ML-KEM-768 + X25519 post-quantum hybrid identity
	// (AGE-SECRET-KEY-PQ-1...), as produced by `age-keygen -pq`.
	KeyTypeHybrid KeyType = "hybrid"
)

// blobVersion is the current payload schema version. Bumped only for
// backwards-incompatible changes; additive fields can be introduced without a
// bump because Decode tolerates unknown keys.
const blobVersion = "1"

// Blob is the decoded contents of a plugin identity payload.
type Blob struct {
	// Account is the 1Password account UUID or account name that the desktop
	// SDK client should be scoped to. It is embedded so the identity file is
	// self-contained and portable across machines signed into the same account.
	Account string
	// Ref is a 1Password secret reference of the form
	// "op://<vault>/<item>/<field>" suitable for Secrets().Resolve.
	Ref string
	// Type is the age key format of the stored secret. Encode/Decode default
	// to KeyTypeX25519 when empty.
	Type KeyType
}

// Encode serializes the blob into the raw bytes that should be passed to
// plugin.EncodeIdentity. The output is deterministic so that regenerating an
// identity file with the same inputs produces byte-identical output.
func (b Blob) Encode() ([]byte, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}
	kt := b.Type
	if kt == "" {
		kt = KeyTypeX25519
	}

	// Deterministic ordering: v first, then the rest sorted alphabetically.
	// This keeps diffs of regenerated identity files stable.
	rest := map[string]string{
		"account": b.Account,
		"ref":     b.Ref,
		"type":    string(kt),
	}
	keys := make([]string, 0, len(rest))
	for k := range rest {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	fmt.Fprintf(&sb, "v=%s\n", blobVersion)
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s=%s\n", k, rest[k])
	}
	return []byte(sb.String()), nil
}

// Decode parses the raw payload bytes produced by Encode (i.e. the data that
// plugin.ParseIdentity returns from an AGE-PLUGIN-1PASS-1... string).
//
// Unknown keys are ignored to leave room for additive evolution. A missing or
// mismatched `v=` header is a hard error so that we can bump the schema later
// without silently misinterpreting old payloads.
func Decode(data []byte) (Blob, error) {
	var b Blob
	var sawVersion bool

	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return Blob{}, fmt.Errorf("identity blob line %d: missing '=' separator", i+1)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "v":
			if val != blobVersion {
				return Blob{}, fmt.Errorf("identity blob: unsupported version %q (want %q)", val, blobVersion)
			}
			sawVersion = true
		case "account":
			b.Account = val
		case "ref":
			b.Ref = val
		case "type":
			switch KeyType(val) {
			case KeyTypeX25519, KeyTypeHybrid:
				b.Type = KeyType(val)
			default:
				return Blob{}, fmt.Errorf("identity blob: unknown key type %q", val)
			}
		default:
			// Ignore unknown keys for forward compatibility.
		}
	}

	if !sawVersion {
		return Blob{}, errors.New("identity blob: missing 'v=' header")
	}
	if b.Type == "" {
		b.Type = KeyTypeX25519
	}
	if err := b.validate(); err != nil {
		return Blob{}, err
	}
	return b, nil
}

func (b Blob) validate() error {
	if b.Account == "" {
		return errors.New("identity blob: account is required")
	}
	if b.Ref == "" {
		return errors.New("identity blob: ref is required")
	}
	if !strings.HasPrefix(b.Ref, "op://") {
		return fmt.Errorf("identity blob: ref %q must start with op://", b.Ref)
	}
	// Newlines in a field would corrupt the line-oriented encoding.
	for name, v := range map[string]string{"account": b.Account, "ref": b.Ref} {
		if strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("identity blob: %s must not contain newlines", name)
		}
	}
	return nil
}
