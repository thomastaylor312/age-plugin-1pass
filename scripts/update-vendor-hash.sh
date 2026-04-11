#!/usr/bin/env bash
#
# Update the `vendorHash` in flake.nix to match the current go.mod / go.sum.
#
# Usage: scripts/update-vendor-hash.sh [flake-path]
#
# Runs `nix build .#age-plugin-1pass`. If the hash is already correct, the
# script exits 0 without touching anything. Otherwise it parses the
# "got: sha256-..." value out of Nix's mismatch error, rewrites the
# `vendorHash = "..."` line in flake.nix, and re-runs the build to confirm.
#
# Intended to be invoked both locally and from the vendor-hash GitHub
# Actions workflow that reacts to Dependabot go-module bumps.

set -euo pipefail

FLAKE="${1:-flake.nix}"
PACKAGE_ATTR=".#age-plugin-1pass"

if [[ ! -f "$FLAKE" ]]; then
  echo "error: $FLAKE not found" >&2
  exit 2
fi

if ! command -v nix >/dev/null 2>&1; then
  echo "error: nix is not installed or not on PATH" >&2
  exit 2
fi

echo "Checking current vendorHash against ${PACKAGE_ATTR}..."

# First attempt: if the current hash already matches, we're done.
if nix build --no-link --print-build-logs "$PACKAGE_ATTR" >/tmp/update-vendor-hash.log 2>&1; then
  echo "vendorHash is already up to date."
  exit 0
fi

# Capture stderr+stdout so we can parse the mismatch message.
build_output=$(cat /tmp/update-vendor-hash.log)

new_hash=$(
  printf '%s\n' "$build_output" \
    | grep -Eo 'got:[[:space:]]+sha256-[A-Za-z0-9+/=]+' \
    | head -n1 \
    | awk '{print $2}'
)

if [[ -z "$new_hash" ]]; then
  echo "error: nix build failed but no vendorHash mismatch was reported." >&2
  echo "------- nix build output -------" >&2
  printf '%s\n' "$build_output" >&2
  exit 1
fi

echo "New vendorHash: $new_hash"

# Rewrite the single `vendorHash = "sha256-...";` line in-place.
# Uses Perl to stay portable between GNU sed (Linux) and BSD sed (macOS).
perl -i -pe \
  "s|vendorHash = \"sha256-[A-Za-z0-9+/=]+\";|vendorHash = \"${new_hash}\";|" \
  "$FLAKE"

echo "Updated $FLAKE; verifying with a second build..."
nix build --no-link --print-build-logs "$PACKAGE_ATTR"
echo "vendorHash updated and verified."
