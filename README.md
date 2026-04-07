# age-plugin-1pass

An [age](https://age-encryption.org) identity plugin that stores private keys
in [1Password](https://1password.com) and resolves them at decrypt time via
the 1Password desktop-app integration. Supports both standard X25519 and
post-quantum ML-KEM-768 + X25519 hybrid keys.

The identity file is self-contained: it embeds the 1Password account and
secret reference needed to fetch the key, so the same file works on any
machine signed into that 1Password account.

## Requirements

- 1Password desktop app, running and signed into the target account.
- **Integrate with other apps** enabled under *Settings → Developer* in the
  1Password desktop app (required for the Go SDK's desktop-app auth).
- [`age`](https://github.com/FiloSottile/age) installed on `PATH`.
- **CGO enabled** at build time — the 1Password Go SDK links native code.

## Install

```bash
CGO_ENABLED=1 go install github.com/thomastaylor312/age-plugin-1pass@latest
```

Make sure `$(go env GOBIN)` (or `$GOPATH/bin`) is on your `PATH` — `age`
discovers plugins by looking for a binary named `age-plugin-1pass`.

## Quick start

### Generate an X25519 identity

```bash
age-plugin-1pass generate \
    --account my.1password.com \
    --vault Personal \
    --name my-age-key \
    -o ~/.config/age/1pass.key
```

This creates a new X25519 keypair, stores the private key in the `password`
field of a new Password-category item called `my-age-key` in your `Personal`
vault, and writes an identity file to `~/.config/age/1pass.key`. The public
key is printed to stderr so you can copy it into `~/.config/age/recipients`
or share it.

### Generate a post-quantum hybrid identity

```bash
age-plugin-1pass generate \
    --account my.1password.com \
    --vault Personal \
    --name my-age-pq-key \
    -pq \
    -o ~/.config/age/1pass-pq.key
```

### Encrypt and decrypt

Encryption uses the regular `age` recipient — no plugin involvement needed:

```bash
PUBKEY=$(grep 'public key' ~/.config/age/1pass.key | awk '{print $NF}')
echo 'hello' | age -r "$PUBKEY" -o hello.age
```

Decryption uses the identity file; `age` spawns `age-plugin-1pass`, which
fetches the secret from 1Password (you may see a desktop-app approval prompt
on first use):

```bash
age -d -i ~/.config/age/1pass.key hello.age
# -> hello
```

## Flags

`age-plugin-1pass generate`:

| Flag | Required | Description |
|------|----------|-------------|
| `--account <id-or-name>` | yes | 1Password account UUID or sidebar name. See "Why is `--account` required?" below. |
| `--vault <name-or-id>` | yes | Destination vault; title or UUID. |
| `--name <item-name>` | no | Item title. Prompted interactively if omitted. |
| `-o, --output <path>` | no | Write to a file (refuses to overwrite) instead of stdout. |
| `-pq` | no | Generate a post-quantum ML-KEM-768 + X25519 hybrid key. |

### Why is `--account` required?

The 1Password Go SDK's desktop-app integration needs a specific account to
bind the client to, and the SDK exposes no API to list or default accounts.
A future release may shell out to `op account list` to auto-select when a
single account is signed in — tracked as a TODO in `internal/cmd/generate.go`.

## Identity file format

A generated identity file looks like:

```
# created: 2026-04-06T22:15:00-07:00
# public key: age1...
# 1password: op://Personal/my-age-key/password
# account: my.1password.com
AGE-PLUGIN-1PASS-1...
```

The last line is a bech32 blob containing a small `key=value` payload:

```
v=1
account=my.1password.com
ref=op://Personal/my-age-key/password
type=x25519
```

You can decode it with any bech32 tool for debugging — it contains only the
account identifier and the 1Password secret reference, never the key itself.

## Troubleshooting

**`desktop app connection channel is closed`** — the 1Password desktop app
isn't running, isn't signed into the requested account, or *Integrate with
other apps* is disabled. Open *Settings → Developer* and toggle it on.

**`vault "X" not found in account "Y"`** — the desktop app is signed into a
different account than `--account`, or the vault title is misspelled. Use
`op vault list` (with the `op` CLI) to confirm titles and IDs.

**Decryption hangs with no prompt** — the desktop app's approval dialog may
be behind another window, or the app is locked. Unlock 1Password and retry.

**`parse X25519 identity from 1Password ref ...`** — the item exists but the
`password` field doesn't contain a valid `AGE-SECRET-KEY-...` string. This
usually means the item was edited or created by hand. Regenerate with
`age-plugin-1pass generate`.

## Development

A `Justfile` wraps the common tasks. A `flake.nix` provides Go 1.26, `just`,
and `golangci-lint` in a reproducible devshell:

```bash
nix develop         # enter the devshell
just check          # gofmt + golangci-lint + go test
just build          # build ./age-plugin-1pass
```

All commits on `main` are expected to pass `just check`.

## License

TBD.
