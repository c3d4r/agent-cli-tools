#!/bin/sh
set -e

REPO="c3d4r/agent-cli-tools"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
case "$(uname -s)" in
    Linux)  OS="linux" ;;
    Darwin) OS="darwin" ;;
    *)      echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

# Detect arch
case "$(uname -m)" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)              echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

# Determine version
if [ -z "$VERSION" ]; then
    VERSION=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
    if [ -z "$VERSION" ]; then
        echo "Failed to determine latest version" >&2
        exit 1
    fi
fi

VERSION_NUM="${VERSION#v}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

echo "Installing agent-cli-tools ${VERSION} (${OS}/${ARCH})"
echo "  Install directory: ${INSTALL_DIR}"

mkdir -p "$INSTALL_DIR"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

for TOOL in e lsp-cli; do
    ARCHIVE="${TOOL}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
    URL="${BASE_URL}/${ARCHIVE}"

    echo "  Downloading ${TOOL}..."
    curl -sL "$URL" -o "${TMPDIR}/${ARCHIVE}"
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"
    mv "${TMPDIR}/${TOOL}" "${INSTALL_DIR}/${TOOL}"
    chmod +x "${INSTALL_DIR}/${TOOL}"
done

echo ""
echo "Installed e and lsp-cli to ${INSTALL_DIR}"

# Check if install dir is in PATH
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "  Note: ${INSTALL_DIR} is not in your PATH. Add it with:"
       echo "    export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
esac
