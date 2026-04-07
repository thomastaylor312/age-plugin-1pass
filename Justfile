# age-plugin-1pass task runner.
#
# The 1Password SDK's desktop-app auth requires cgo, so every build/test
# recipe exports CGO_ENABLED=1. Override by running `go` directly if you
# need a pure-Go build for experimentation.

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

export CGO_ENABLED := "1"

# Default target lists available recipes.
default:
    @just --list

# Build the plugin binary into ./age-plugin-1pass.
build:
    go build -o age-plugin-1pass .

# Run the unit test suite.
test:
    go test ./...

# Run golangci-lint.
lint:
    golangci-lint run

# Format sources in-place.
fmt:
    gofmt -s -w .

# Run format, lint, and tests — the standard pre-commit gate.
check: fmt lint test

# Remove build artifacts.
clean:
    rm -f age-plugin-1pass
