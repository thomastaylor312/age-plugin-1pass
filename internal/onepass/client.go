// Package onepass wraps the 1Password Go SDK with just the operations that
// age-plugin-1pass needs: resolving a secret reference at decrypt time, and
// creating a Password-category item at generate time. It exists so that the
// rest of the codebase can depend on small interfaces instead of the full SDK
// surface (which pulls in cgo + wazero and is awkward to mock).
//
// Authentication uses the new desktop-app integration exclusively. See the
// TODO on DesktopClient for the service-account fallback we've explicitly
// deferred.
package onepass

import (
	"context"
	"fmt"

	onepassword "github.com/1password/onepassword-sdk-go"
)

// IntegrationName is sent to 1Password as part of the client handshake and
// shows up in audit logs. Keep it stable so users can recognize entries.
const IntegrationName = "age-plugin-1pass"

// DesktopClient is a thin facade over *onepassword.Client scoped to a single
// account via the desktop-app integration.
//
// TODO(account-autodetect): the SDK does not expose a way to list accounts or
// discover a default, so callers must currently supply --account explicitly.
// A future enhancement could shell out to `op account list --format=json` (or
// honor OP_ACCOUNT) to auto-select when unambiguous.
type DesktopClient struct {
	inner   *onepassword.Client
	account string
}

// NewDesktop builds a client bound to the given 1Password account. The
// accountName may be an account UUID or a shorthand name from the desktop
// app sidebar. The desktop app must be running and signed into that account;
// otherwise NewClient returns an error that we surface as-is.
func NewDesktop(ctx context.Context, account, version string) (*DesktopClient, error) {
	if account == "" {
		return nil, fmt.Errorf("onepass: account is required")
	}
	c, err := onepassword.NewClient(ctx,
		onepassword.WithDesktopAppIntegration(account),
		onepassword.WithIntegrationInfo(IntegrationName, version),
	)
	if err != nil {
		return nil, fmt.Errorf("onepass: initialize desktop client for account %q: %w", account, err)
	}
	return &DesktopClient{inner: c, account: account}, nil
}

// Account returns the account identifier the client was constructed with.
func (c *DesktopClient) Account() string { return c.account }

// Resolve implements identity.SecretResolver by delegating to
// Secrets().Resolve. The ref must be an "op://vault/item/field" string.
func (c *DesktopClient) Resolve(ctx context.Context, ref string) (string, error) {
	return c.inner.Secrets().Resolve(ctx, ref)
}

// ResolveVault looks up a vault by name or ID and returns the canonical vault
// title along with its UUID. We need both: the UUID goes into ItemCreateParams
// and the title goes into the op:// reference that we bake into the identity
// file (references resolve fine against titles and are more human-readable
// than UUIDs).
func (c *DesktopClient) ResolveVault(ctx context.Context, nameOrID string) (title, id string, err error) {
	vaults, err := c.inner.Vaults().List(ctx)
	if err != nil {
		return "", "", fmt.Errorf("onepass: list vaults: %w", err)
	}
	for _, v := range vaults {
		if v.ID == nameOrID || v.Title == nameOrID {
			return v.Title, v.ID, nil
		}
	}
	return "", "", fmt.Errorf("onepass: vault %q not found in account %q", nameOrID, c.account)
}

// CreatePasswordItem stores `secret` in the built-in `password` field of a
// new Password-category item. The field uses the standard ID "password" so
// that the resulting op reference "op://<vault>/<item>/password" resolves
// without needing a custom section. Returns the created item's title (which
// may differ from the requested title if the server de-duplicates, though
// 1Password normally preserves it).
func (c *DesktopClient) CreatePasswordItem(ctx context.Context, vaultID, title, secret string) (string, error) {
	item, err := c.inner.Items().Create(ctx, onepassword.ItemCreateParams{
		Category: onepassword.ItemCategoryPassword,
		VaultID:  vaultID,
		Title:    title,
		Fields: []onepassword.ItemField{{
			ID:        "password",
			Title:     "password",
			FieldType: onepassword.ItemFieldTypeConcealed,
			Value:     secret,
		}},
	})
	if err != nil {
		return "", fmt.Errorf("onepass: create item %q in vault %q: %w", title, vaultID, err)
	}
	return item.Title, nil
}
