#!/bin/sh
set -e

REPO="JLugagne/claude-mercato"
BINARY="mct"
INSTALL_DIR="${MCT_INSTALL_DIR:-/usr/local/bin}"

# Resolve OS
OS="$(uname -s)"
case "$OS" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

# Resolve arch
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

# Resolve latest version
if [ -z "$MCT_VERSION" ]; then
  MCT_VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

if [ -z "$MCT_VERSION" ]; then
  echo "Could not determine latest version. Set MCT_VERSION to override." >&2
  exit 1
fi

ARCHIVE="mct_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${MCT_VERSION}/${ARCHIVE}"

echo "Installing mct ${MCT_VERSION} (${OS}/${ARCH})..."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/$ARCHIVE"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

# Install (use sudo if needed)
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi

chmod +x "$INSTALL_DIR/$BINARY"

echo "mct installed to $INSTALL_DIR/$BINARY"
"$INSTALL_DIR/$BINARY" --version
