// Package cmd contains the entrypoints for each mode age-plugin-1pass runs
// in: the age plugin state machine (identity-v1) and the human-facing
// subcommands like `generate`.
package cmd

import (
	"context"
	"fmt"
	"os"

	"filippo.io/age"
	ageplugin "filippo.io/age/plugin"

	"github.com/thomastaylor312/age-plugin-1pass/internal/identity"
	"github.com/thomastaylor312/age-plugin-1pass/internal/onepass"
)

// PluginName is the name registered with filippo.io/age/plugin. It determines
// the bech32 HRPs used for identities/recipients (AGE-PLUGIN-1PASS-...) and
// the binary name age expects on PATH (age-plugin-1pass).
const PluginName = "1pass"

// RunPlugin runs the age plugin protocol. It is invoked when age spawns us
// with --age-plugin=identity-v1. The returned int is an exit code.
//
// Each identity handler allocates its own DesktopClient on first Unwrap so
// that we only authenticate against 1Password when age actually needs a
// file key — listing identities with `age -i` should not trigger a prompt.
func RunPlugin(version string) int {
	p, err := ageplugin.New(PluginName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "age-plugin-1pass: plugin init: %v\n", err)
		return 1
	}

	p.HandleIdentity(func(data []byte) (age.Identity, error) {
		blob, err := identity.Decode(data)
		if err != nil {
			return nil, fmt.Errorf("decode identity payload: %w", err)
		}

		// The age plugin protocol has no dedicated context; use a
		// background context scoped to this plugin process. Lifetime
		// is bounded by the age invocation that spawned us.
		ctx := context.Background()
		client, err := onepass.NewDesktop(ctx, blob.Account, version)
		if err != nil {
			return nil, err
		}
		return identity.New(ctx, blob, client)
	})

	return p.Main()
}
