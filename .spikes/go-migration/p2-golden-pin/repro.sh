#!/usr/bin/env bash
# Reproducer for S7A — golden-pin: download + SHA-verify v1 release assets.
# Mirrors what go/test/golden/harness_test.go will do in S7B.
#
# Usage:  ./repro.sh [darwin-arm64|darwin-x64|linux-arm64|linux-x64]
#         (default: linux-x64 — matches CI host platform)
set -euo pipefail

PLATFORM="${1:-linux-x64}"
TAG="v1.0.0"
REPO="hungthai1401/hostie"
ASSET="hostie-${PLATFORM}"
BASE="https://github.com/${REPO}/releases/download/${TAG}"

cache_dir="$(cd "$(dirname "$0")" && pwd)/.cache/${TAG}/${PLATFORM}"
mkdir -p "$cache_dir"
cd "$cache_dir"

# 1. Fetch sidecar (.sha256). This is the authoritative expected hash.
if [ ! -f "${ASSET}.sha256" ]; then
  echo "==> Fetching ${ASSET}.sha256"
  curl -fsSL --max-time 30 -o "${ASSET}.sha256" "${BASE}/${ASSET}.sha256"
fi

# 2. Fetch binary (large — may take time).
if [ ! -f "${ASSET}" ]; then
  echo "==> Fetching ${ASSET}"
  curl -fL --max-time 300 -o "${ASSET}" "${BASE}/${ASSET}"
fi

# 3. Verify — refuse loudly on mismatch.
echo "==> Verifying SHA-256"
if shasum -a 256 -c "${ASSET}.sha256"; then
  chmod +x "${ASSET}"
  echo "==> OK: ${cache_dir}/${ASSET}"
  exit 0
else
  echo "==> SHA MISMATCH — refusing. Removing cached binary."
  rm -f "${ASSET}"
  exit 1
fi
