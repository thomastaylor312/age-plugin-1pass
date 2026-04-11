package cmd

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"filippo.io/age"
	ageplugin "filippo.io/age/plugin"

	"github.com/thomastaylor312/age-plugin-1pass/internal/identity"
	"github.com/thomastaylor312/age-plugin-1pass/internal/onepass"
)

// GenerateFlags captures the CLI surface of the `generate` subcommand.
type GenerateFlags struct {
	Account string
	Vault   string
	Name    string
	Output  string
	PQ      bool
}

// RunGenerate parses `generate` arguments and executes the subcommand.
func RunGenerate(args []string, version string) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var f GenerateFlags
	// TODO(account-autodetect): The 1Password Go SDK has no accounts-list API, so --account is
	// required today.
	// Pass the shorthand/email shown in the desktop app's account picker — the 26-char account ID
	// from `op account list` does NOT work here, despite what the SDK godoc suggests.
	fs.StringVar(&f.Account, "account", "", "1Password account name (sidebar shorthand/email; NOT the account ID) (prompted if empty)")
	fs.StringVar(&f.Vault, "vault", "", "1Password vault name or ID (prompted if empty)")
	fs.StringVar(&f.Name, "name", "", "item title for the 1Password entry (prompted if empty)")
	fs.StringVar(&f.Output, "o", "", "write the identity file to this path instead of stdout")
	fs.StringVar(&f.Output, "output", "", "write the identity file to this path instead of stdout")
	fs.BoolVar(&f.PQ, "pq", false, "generate a post-quantum ML-KEM-768 + X25519 hybrid key")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: age-plugin-1pass generate [--account <acct>] [--vault <vault>] [--name <name>] [-o FILE] [-pq]

Generates a new age key, stores the private key in 1Password as a
Password-category item, and writes a self-contained identity file that
can later be used with `+"`age -d -i <file>`"+`.
`)
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := runGenerate(f, version, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "age-plugin-1pass: %v\n", err)
		return 1
	}
	return 0
}

// runGenerate is the testable core of the subcommand. It's kept separate so future tests can drive
// it with fake stdin/stdout and a mocked 1Password layer (via an interface introduced when we need
// it).
func runGenerate(f GenerateFlags, version string, stdin io.Reader, stdout, stderr io.Writer) error {
	// Refuse to overwrite an existing output file. age-keygen does the
	// same — losing a private-key file to a stray redirect is catastrophic.
	if f.Output != "" {
		if _, err := os.Stat(f.Output); err == nil {
			return fmt.Errorf("output file %q already exists; refusing to overwrite", f.Output)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat output file %q: %w", f.Output, err)
		}
	}

	// Share a single bufio.Reader across prompts — constructing a new one
	// per call can drop input that the previous reader buffered past the
	// newline.
	reader := bufio.NewReader(stdin)
	account, err := promptIfEmpty(f.Account, "1Password account (shorthand or email from sidebar)", "account is required", reader, stderr)
	if err != nil {
		return err
	}
	vault, err := promptIfEmpty(f.Vault, "1Password vault name or ID", "vault is required", reader, stderr)
	if err != nil {
		return err
	}
	name, err := promptIfEmpty(f.Name, "1Password item name", "item name is required", reader, stderr)
	if err != nil {
		return err
	}

	// Generate the key locally first
	var (
		secretStr    string
		recipientStr string
		keyType      identity.KeyType
	)
	if f.PQ {
		id, err := age.GenerateHybridIdentity()
		if err != nil {
			return fmt.Errorf("generate hybrid identity: %w", err)
		}
		secretStr = id.String()
		recipientStr = id.Recipient().String()
		keyType = identity.KeyTypeHybrid
	} else {
		id, err := age.GenerateX25519Identity()
		if err != nil {
			return fmt.Errorf("generate X25519 identity: %w", err)
		}
		secretStr = id.String()
		recipientStr = id.Recipient().String()
		keyType = identity.KeyTypeX25519
	}

	ctx := context.Background()
	client, err := onepass.NewDesktop(ctx, account, version)
	if err != nil {
		return err
	}

	vaultTitle, vaultID, err := client.ResolveVault(ctx, vault)
	if err != nil {
		return err
	}

	itemTitle, err := client.CreatePasswordItem(ctx, vaultID, name, secretStr)
	if err != nil {
		return err
	}

	blob := identity.Blob{
		Account: account,
		Ref:     fmt.Sprintf("op://%s/%s/password", vaultTitle, itemTitle),
		Type:    keyType,
	}
	payload, err := blob.Encode()
	if err != nil {
		return fmt.Errorf("encode identity blob: %w", err)
	}
	pluginIdentity := ageplugin.EncodeIdentity(PluginName, payload)

	// Identity file format mirrors age-keygen with extra 1Password
	// provenance comments so operators can see at a glance which account
	// and item a given file points at without decoding the bech32 blob.
	var body strings.Builder
	fmt.Fprintf(&body, "# created: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&body, "# public key: %s\n", recipientStr)
	fmt.Fprintf(&body, "# 1password: %s\n", blob.Ref)
	fmt.Fprintf(&body, "# account: %s\n", blob.Account)
	fmt.Fprintf(&body, "%s\n", pluginIdentity)

	if f.Output == "" {
		if _, err := io.WriteString(stdout, body.String()); err != nil {
			return fmt.Errorf("write identity to stdout: %w", err)
		}
		// Also echo the public key to stderr so users can pipe the
		// identity to a file while still seeing the recipient.
		_, _ = fmt.Fprintf(stderr, "Public key: %s\n", recipientStr)
		return nil
	}

	// Write with 0600 perms — this file embeds the info needed to fetch
	// a private key from 1Password; treat it as sensitive.
	if err := os.WriteFile(f.Output, []byte(body.String()), 0o600); err != nil {
		return fmt.Errorf("write identity file %q: %w", f.Output, err)
	}
	_, err = fmt.Fprintf(stderr, "Public key: %s\n", recipientStr)
	if err != nil {
		// Writing to stderr should never fail, but in the weird chance we get to the point, still
		// return an error so at least the command returns non-zero
		return err
	}
	_, err = fmt.Fprintf(stderr, "Identity written to %s\n", f.Output)
	if err != nil {
		return err
	}
	return nil
}

// promptIfEmpty returns flagValue when set, otherwise prompts on stderr and reads a line from
// reader. Uses bufio directly (not age/plugin's RequestValue) because we're outside plugin mode.
// The caller supplies the reader so multiple prompts can share one bufio buffer.
func promptIfEmpty(flagValue, prompt, missingErr string, reader *bufio.Reader, stderr io.Writer) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	_, _ = fmt.Fprintf(stderr, "%s: ", prompt)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read %s: %w", prompt, err)
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return "", errors.New(missingErr)
	}
	return value, nil
}
