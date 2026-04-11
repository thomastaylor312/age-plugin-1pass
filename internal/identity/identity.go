package identity

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"filippo.io/age"
)

// SecretResolver fetches a secret value by its 1Password reference
// (e.g. "op://Personal/my-age-key/password"). Implementations must be
// safe for concurrent use. The interface lets us inject a fake in unit
// tests so we never touch the real 1Password SDK during `go test`.
type SecretResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// OnePassIdentity is an age.Identity whose underlying secret lives in
// 1Password. The first call to Unwrap resolves the secret via the
// injected SecretResolver, parses it into the appropriate age identity
// type (X25519 or Hybrid/PQ), and caches the result for subsequent
// calls within the same plugin invocation.
//
// The zero value is not usable — construct instances via New.
type OnePassIdentity struct {
	blob     Blob
	resolver SecretResolver
	ctx      context.Context

	once    func() error
	wrapped age.Identity
}

// New returns an OnePassIdentity backed by the supplied resolver. The
// context is stored so that Unwrap (which has no context parameter on
// the age.Identity interface) can still be cancellation-aware — callers
// should pass the plugin's request-scoped context.
func New(ctx context.Context, blob Blob, resolver SecretResolver) (*OnePassIdentity, error) {
	if resolver == nil {
		return nil, errors.New("onepass identity: resolver must not be nil")
	}
	id := &OnePassIdentity{blob: blob, resolver: resolver, ctx: ctx}
	id.once = sync.OnceValue(id.load)
	return id, nil
}

// Unwrap implements age.Identity. It lazily fetches the private key from
// 1Password on first invocation and delegates subsequent decryption to the
// underlying age identity type. Errors from the 1Password SDK (auth, network,
// missing item) are surfaced as-is so the age plugin protocol can report a
// useful message; a mismatched stanza, by contrast, is left to the wrapped
// identity's own Unwrap to signal via age.ErrIncorrectIdentity.
func (i *OnePassIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	if err := i.once(); err != nil {
		return nil, err
	}
	return i.wrapped.Unwrap(stanzas)
}

func (i *OnePassIdentity) load() error {
	secret, err := i.resolver.Resolve(i.ctx, i.blob.Ref)
	if err != nil {
		return fmt.Errorf("resolve 1Password secret %q: %w", i.blob.Ref, err)
	}

	switch i.blob.Type {
	case KeyTypeX25519, "":
		id, err := age.ParseX25519Identity(secret)
		if err != nil {
			return fmt.Errorf("parse X25519 identity from 1Password ref %q: %w", i.blob.Ref, err)
		}
		i.wrapped = id
	case KeyTypeHybrid:
		id, err := age.ParseHybridIdentity(secret)
		if err != nil {
			return fmt.Errorf("parse hybrid identity from 1Password ref %q: %w", i.blob.Ref, err)
		}
		i.wrapped = id
	default:
		return fmt.Errorf("onepass identity: unsupported key type %q", i.blob.Type)
	}
	return nil
}
