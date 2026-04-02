#!/usr/bin/env bash
# Sandkasten installer — downloads the latest release binary and installs it.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/p-arndt/sandkasten/main/scripts/install.sh | sudo bash
#
# Options (via env vars):
#   SANDKASTEN_VERSION=v0.3.0   Install a specific version (default: latest)
#   INSTALL_DIR=/usr/local/bin  Installation directory (default: /usr/local/bin)
#
set -euo pipefail

REPO="p-arndt/sandkasten"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${SANDKASTEN_VERSION:-}"

# Colors (disabled if not a terminal).
if [ -t 1 ]; then
  GREEN='\033[0;32m'
  YELLOW='\033[0;33m'
  RED='\033[0;31m'
  BOLD='\033[1m'
  NC='\033[0m'
else
  GREEN='' YELLOW='' RED='' BOLD='' NC=''
fi

info()  { echo -e "${GREEN}[info]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[warn]${NC}  $*"; }
error() { echo -e "${RED}[error]${NC} $*" >&2; }
die()   { error "$@"; exit 1; }

# Detect OS and architecture.
detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  case "$OS" in
    linux)  ;;
    *)      die "Sandkasten requires Linux (or WSL2). Detected: $OS" ;;
  esac

  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)             die "Unsupported architecture: $ARCH" ;;
  esac
}

# Resolve the latest version tag from GitHub.
resolve_version() {
  if [ -n "$VERSION" ]; then
    info "Using specified version: $VERSION"
    return
  fi

  info "Fetching latest release..."
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | head -1 \
    | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')"

  if [ -z "$VERSION" ]; then
    die "Could not determine latest version. Set SANDKASTEN_VERSION manually."
  fi

  info "Latest version: $VERSION"
}

# Download and install the binary.
install_binary() {
  BINARY_NAME="sandkasten-${OS}-${ARCH}"
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"

  info "Downloading ${BINARY_NAME}..."
  TMP="$(mktemp)"
  HTTP_CODE="$(curl -fsSL -w '%{http_code}' -o "$TMP" "$URL" 2>/dev/null || true)"

  if [ "$HTTP_CODE" != "200" ] || [ ! -s "$TMP" ]; then
    rm -f "$TMP"
    die "Download failed (HTTP $HTTP_CODE). Check that version $VERSION exists at:\n  $URL"
  fi

  chmod +x "$TMP"

  # Install to target directory.
  mkdir -p "$INSTALL_DIR"
  mv "$TMP" "${INSTALL_DIR}/sandkasten"

  info "Installed to ${INSTALL_DIR}/sandkasten"
}

# Verify the installation.
verify() {
  if command -v sandkasten &>/dev/null; then
    INSTALLED_VERSION="$(sandkasten version 2>/dev/null || echo "unknown")"
    info "Verified: $INSTALLED_VERSION"
  elif [ -x "${INSTALL_DIR}/sandkasten" ]; then
    info "Binary installed at ${INSTALL_DIR}/sandkasten"
    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
      warn "${INSTALL_DIR} is not in your PATH. Add it with:"
      echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
  fi
}

print_next_steps() {
  echo ""
  echo -e "${BOLD}Sandkasten installed successfully!${NC}"
  echo ""
  echo "  Quick start (zero config):"
  echo "    sudo sandkasten up"
  echo ""
  echo "  This will:"
  echo "    - Check your environment"
  echo "    - Pull a Python sandbox image"
  echo "    - Generate an API key"
  echo "    - Start the daemon on localhost:8080"
  echo ""
  echo "  Or use Docker:"
  echo "    docker run -d --privileged -p 8080:8080 ghcr.io/p-arndt/sandkasten:latest"
  echo ""
}

main() {
  detect_platform
  resolve_version
  install_binary
  verify
  print_next_steps
}

main "$@"
