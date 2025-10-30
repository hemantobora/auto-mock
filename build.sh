#!/usr/bin/env bash
set -euo pipefail

# Build script for AutoMock CLI
# Usage:
#   ./build.sh                # builds with version from git or 0.1.0
#   VERSION=1.0.0 ./build.sh  # override version

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

# Determine version
if [[ -z "${VERSION:-}" ]]; then
  if command -v git >/dev/null 2>&1 && git rev-parse --git-dir >/dev/null 2>&1; then
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
  else
    VERSION="0.1.0"
  fi
fi

echo "Building AutoMock version: $VERSION"
GOFLAGS=${GOFLAGS:-}
GOOS=${GOOS:-}
GOARCH=${GOARCH:-}

# Output binary name
OUT=${OUT:-automock}

# Build command
cmd=(go build -ldflags "-X main.version=$VERSION" -o "$OUT" ./cmd/auto-mock)

# Allow cross-compilation via GOOS/GOARCH
if [[ -n "${GOOS}" ]]; then export GOOS; fi
if [[ -n "${GOARCH}" ]]; then export GOARCH; fi

"${cmd[@]}"

echo "✅ Built ./$OUT"
