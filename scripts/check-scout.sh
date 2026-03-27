#!/usr/bin/env bash
# Scout plugin: verify or install the scout binary.
set -euo pipefail

REPO="inovacc/scout"
PLUGIN_DATA="${CLAUDE_PLUGIN_DATA:-${HOME}/.claude/plugins/data/scout}"
BIN_DIR="${PLUGIN_DATA}/bin"

# Check if scout is already available
if command -v scout &>/dev/null; then
    version=$(scout version 2>/dev/null || echo "unknown")
    echo "scout plugin: binary found (${version})"
    exit 0
fi

# Check if we previously downloaded it
if [ -x "${BIN_DIR}/scout" ] || [ -x "${BIN_DIR}/scout.exe" ]; then
    export PATH="${BIN_DIR}:${PATH}"
    version=$(scout version 2>/dev/null || echo "unknown")
    echo "scout plugin: using cached binary (${version})"
    exit 0
fi

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "${ARCH}" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "scout plugin: unsupported architecture ${ARCH}"; exit 0 ;;
esac

# Map OS name
case "${OS}" in
    darwin|linux) ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo "scout plugin: unsupported OS ${OS}"; exit 0 ;;
esac

EXT="tar.gz"
[ "${OS}" = "windows" ] && EXT="zip"

echo "scout plugin: downloading binary for ${OS}/${ARCH}..."

# Get latest release tag
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
if [ -z "${TAG}" ]; then
    echo "scout plugin: could not determine latest release"
    echo "  Install manually: go install github.com/inovacc/scout/cmd/scout@latest"
    exit 0
fi

VERSION="${TAG#v}"
ARCHIVE="scout_${VERSION}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

mkdir -p "${BIN_DIR}"
TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

if ! curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "${URL}"; then
    echo "scout plugin: download failed from ${URL}"
    echo "  Install manually: go install github.com/inovacc/scout/cmd/scout@latest"
    exit 0
fi

# Extract
if [ "${EXT}" = "zip" ]; then
    unzip -q "${TMPDIR}/${ARCHIVE}" -d "${TMPDIR}/extracted"
else
    mkdir -p "${TMPDIR}/extracted"
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "${TMPDIR}/extracted"
fi

# Find and install binary
BINARY=$(find "${TMPDIR}/extracted" -name "scout" -o -name "scout.exe" | head -1)
if [ -z "${BINARY}" ]; then
    echo "scout plugin: binary not found in archive"
    exit 0
fi

cp "${BINARY}" "${BIN_DIR}/"
chmod +x "${BIN_DIR}/scout" 2>/dev/null || true

export PATH="${BIN_DIR}:${PATH}"
version=$(scout version 2>/dev/null || echo "${TAG}")
echo "scout plugin: installed ${version} to ${BIN_DIR}"
