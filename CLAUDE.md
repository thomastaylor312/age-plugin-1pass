# age-plugin-1pass

An age plugin that stores and retrieves private keys in 1Password via the
desktop app's SDK integration.

## Requirements

A task is not complete until **all tests and lints pass**. Before declaring
any change done:

- Run `just check` (or at minimum `just fmt`, `just lint`, and `just test`)
  and confirm every step exits cleanly.
- Fix failures rather than skipping, disabling, or working around tests or
  lint rules.

## `just` commands

The `Justfile` is the canonical entry point for building and testing. Every
recipe exports `CGO_ENABLED=1` because the 1Password SDK's desktop-app auth
path requires cgo — drop to `go` directly only if you explicitly want a
pure-Go build.

- `just` — list all recipes (default target).
- `just build` — build the plugin binary to `./age-plugin-1pass`.
- `just test` — run the unit test suite (`go test ./...`).
- `just lint` — run `golangci-lint run`.
- `just fmt` — format sources in-place with `gofmt -s -w .`.
- `just check` — run `fmt`, `lint`, and `test` in sequence. This is the
  standard pre-commit gate; run it before sending a change.
- `just clean` — remove the built `age-plugin-1pass` binary.
