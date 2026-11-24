#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BACKEND_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(CDPATH= cd -- "$BACKEND_DIR/.." && pwd)"
VERSION_FILE="$BACKEND_DIR/cmd/server/VERSION"

# A customized release must never inherit an upstream tag's plain version.
FORK_VERSION="$(tr -d '\r\n' < "$VERSION_FILE")"
if printf '%s\n' "$FORK_VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+-mr-[0-9]{6}w[0-9]{2,}$'; then
  printf '%s\n' "$FORK_VERSION"
  exit 0
fi

# Prefer the exact release tag when building from a tagged checkout so
# source builds from vX.Y.Z don't inherit the previous VERSION file value.
if command -v git >/dev/null 2>&1; then
  TAG="$(
    git -C "$REPO_DIR" describe --tags --exact-match --match 'v[0-9]*' 2>/dev/null || \
    git -C "$REPO_DIR" describe --tags --exact-match --match '[0-9]*' 2>/dev/null || \
    true
  )"
  if [ -n "$TAG" ]; then
    printf '%s\n' "${TAG#v}"
    exit 0
  fi
fi

printf '%s\n' "$(tr -d '\r\n' < "$VERSION_FILE")"
