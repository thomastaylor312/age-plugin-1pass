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

	once    sync.Once
	wrapped age.Identity
	err     error
}

// New returns an OnePassIdentity backed by the supplied resolver. The
// context is stored so that Unwrap (which has no context parameter on
// the age.Identity interface) can still be cancellation-aware — callers
// should pass the plugin's request-scoped context.
func New(ctx context.Context, blob Blob, resolver SecretResolver) (*OnePassIdentity, error) {
	if resolver == nil {
		return nil, errors.New("onepass identity: resolver must not be nil")
	}
	return &OnePassIdentity{blob: blob, resolver: resolver, ctx: ctx}, nil
}

// Unwrap implements age.Identity. It lazily fetches the private key from
// 1Password on first invocation and delegates subsequent decryption to the
// underlying age identity type. Errors from the 1Password SDK (auth, network,
// missing item) are surfaced as-is so the age plugin protocol can report a
// useful message; a mismatched stanza, by contrast, is left to the wrapped
// identity's own Unwrap to signal via age.ErrIncorrectIdentity.
func (i *OnePassIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	i.once.Do(i.load)
	if i.err != nil {
		return nil, i.err
	}
	return i.wrapped.Unwrap(stanzas)
}

func (i *OnePassIdentity) load() {
	secret, err := i.resolver.Resolve(i.ctx, i.blob.Ref)
	if err != nil {
		i.err = fmt.Errorf("resolve 1Password secret %q: %w", i.blob.Ref, err)
		return
	}

	switch i.blob.Type {
	case KeyTypeX25519, "":
		id, err := age.ParseX25519Identity(secret)
		if err != nil {
			i.err = fmt.Errorf("parse X25519 identity from 1Password ref %q: %w", i.blob.Ref, err)
			return
		}
		i.wrapped = id
	case KeyTypeHybrid:
		id, err := age.ParseHybridIdentity(secret)
		if err != nil {
			i.err = fmt.Errorf("parse hybrid identity from 1Password ref %q: %w", i.blob.Ref, err)
			return
		}
		i.wrapped = id
	default:
		i.err = fmt.Errorf("onepass identity: unsupported key type %q", i.blob.Type)
	}
}
