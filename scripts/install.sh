#!/bin/sh
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

REPO="pavlitoss/scout-cli"
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$VERSION" ]; then
  echo "Could not determine latest version."
  exit 1
fi

BINARY="scout-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

echo "Installing scout ${VERSION} (${OS}/${ARCH}) to ${INSTALL_DIR}..."
curl -fsSL "$URL" -o "${INSTALL_DIR}/scout"
chmod +x "${INSTALL_DIR}/scout"
echo "Done. Run 'scout --help' to get started."
