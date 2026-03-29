#!/usr/bin/env bash
# Scout installer for Linux and macOS.
# Usage: curl -fsSL https://raw.githubusercontent.com/inovacc/scout/main/scripts/install.sh | bash
set -euo pipefail

REPO="inovacc/scout"
APP_DIR="${SCOUT_INSTALL_DIR:-${HOME}/.scout}"
BIN_DIR="${SCOUT_BIN_DIR:-/usr/local/bin}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Get latest release tag.
echo "Fetching latest release..."
TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')"
if [ -z "$TAG" ]; then
  echo "Failed to determine latest release." >&2
  exit 1
fi
VERSION="${TAG#v}"
echo "Latest release: $TAG"

# Download archive.
ASSET="scout_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMPDIR}/${ASSET}"

echo "Extracting..."
tar xzf "${TMPDIR}/${ASSET}" -C "$TMPDIR"

# Install binary to app dir.
mkdir -p "$APP_DIR"
cp "${TMPDIR}/scout" "${APP_DIR}/scout"
chmod +x "${APP_DIR}/scout"

# Copy plugin assets from repo (downloaded separately).
PLUGIN_ASSETS="https://api.github.com/repos/${REPO}/contents"
for item in .claude-plugin .mcp.json skills agents hooks; do
  if [ -e "${TMPDIR}/${item}" ]; then
    cp -r "${TMPDIR}/${item}" "${APP_DIR}/${item}"
  fi
done

# Symlink binary for CLI access.
if [ -w "$BIN_DIR" ]; then
  ln -sf "${APP_DIR}/scout" "${BIN_DIR}/scout"
else
  echo "Linking to ${BIN_DIR} (requires sudo)..."
  sudo ln -sf "${APP_DIR}/scout" "${BIN_DIR}/scout"
fi

echo ""
echo "✓ Installed scout ${TAG}"
echo "  Binary: ${APP_DIR}/scout"
echo ""
echo "Run 'scout setup' to configure your AI coding assistant."
