#!/usr/bin/env bash
set -euo pipefail

# jig-mcp installer for Linux and macOS
# Usage: ./install.sh [--prefix /usr/local] [--config-dir /usr/local/jig-mcp]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PREFIX="/usr/local"
INSTALL_ROOT="$PREFIX/jig-mcp"

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Install jig-mcp to a single directory with bin/, tools/, scripts/, and .env.

Options:
  --prefix DIR       Install to DIR/jig-mcp (default: /usr/local/jig-mcp)
  --config-dir DIR   Override install root (default: $PREFIX/jig-mcp)
  --uninstall        Remove jig-mcp installation
  -h, --help         Show this help message

Examples:
  sudo ./install.sh                          # Install to /usr/local/jig-mcp
  ./install.sh --prefix ~/.local             # Install to ~/.local/jig-mcp (no sudo)
  sudo ./install.sh --uninstall              # Remove installation
EOF
    exit 0
}

uninstall() {
    echo "Uninstalling jig-mcp..."
    if [ -d "$INSTALL_ROOT" ]; then
        rm -rf "$INSTALL_ROOT"
        echo "  Removed $INSTALL_ROOT"
    fi
    # Also remove symlink if it exists
    if [ -L "$PREFIX/bin/jig-mcp" ]; then
        rm -f "$PREFIX/bin/jig-mcp"
        echo "  Removed symlink $PREFIX/bin/jig-mcp"
    fi
    echo "Done."
    exit 0
}

UNINSTALL=false

while [ $# -gt 0 ]; do
    case "$1" in
        --prefix)    PREFIX="$2"; shift 2 ;;
        --config-dir) INSTALL_ROOT="$2"; shift 2 ;;
        --uninstall) UNINSTALL=true; shift ;;
        -h|--help)   usage ;;
        *)           echo "Unknown option: $1"; usage ;;
    esac
done

if [ "$UNINSTALL" = true ]; then
    uninstall
fi

# Check that the binary exists in the archive
if [ ! -f "$SCRIPT_DIR/bin/jig-mcp" ]; then
    echo "Error: jig-mcp binary not found in $SCRIPT_DIR/bin/"
    echo "Make sure you extracted the full archive before running install.sh"
    exit 1
fi

echo "Installing jig-mcp..."
echo "  Install root: $INSTALL_ROOT/"
echo ""

# Create install root directory
mkdir -p "$INSTALL_ROOT"

# Install binary to bin/
mkdir -p "$INSTALL_ROOT/bin"
cp "$SCRIPT_DIR/bin/jig-mcp" "$INSTALL_ROOT/bin/jig-mcp"
chmod 755 "$INSTALL_ROOT/bin/jig-mcp"
echo "  Installed binary to $INSTALL_ROOT/bin/jig-mcp"

# Install tools to tools/
if [ -d "$SCRIPT_DIR/tools" ]; then
    cp -r "$SCRIPT_DIR/tools" "$INSTALL_ROOT/tools"
    echo "  Installed example tools to $INSTALL_ROOT/tools/"
fi

# Install scripts to scripts/
if [ -d "$SCRIPT_DIR/scripts" ]; then
    cp -r "$SCRIPT_DIR/scripts" "$INSTALL_ROOT/scripts"
    chmod +x "$INSTALL_ROOT/scripts/"*.sh 2>/dev/null || true
    echo "  Installed example scripts to $INSTALL_ROOT/scripts/"
fi

# Install .env config file
if [ -f "$SCRIPT_DIR/.env" ]; then
    cp "$SCRIPT_DIR/.env" "$INSTALL_ROOT/.env"
    echo "  Installed config template to $INSTALL_ROOT/.env"
fi

# Copy docs
if [ -d "$SCRIPT_DIR/docs" ]; then
    cp -r "$SCRIPT_DIR/docs" "$INSTALL_ROOT/docs"
    echo "  Installed docs to $INSTALL_ROOT/docs/"
fi

# Create logs directory
mkdir -p "$INSTALL_ROOT/logs"

# Create symlink in PREFIX/bin for PATH convenience
ln -sf "$INSTALL_ROOT/bin/jig-mcp" "$PREFIX/bin/jig-mcp"
echo "  Created symlink $PREFIX/bin/jig-mcp -> $INSTALL_ROOT/bin/jig-mcp"

echo ""
echo "Installation complete!"
echo ""
echo "Quick start:"
echo "  cd $INSTALL_ROOT && ./bin/jig-mcp"
echo ""
echo "Or with SSE transport:"
echo "  cd $INSTALL_ROOT && ./bin/jig-mcp -transport sse -port 3001"
echo ""
echo "Or run from anywhere (binary auto-detects config location):"
echo "  jig-mcp"
echo ""

# Check if binary is on PATH
if ! command -v jig-mcp &>/dev/null; then
    echo "Note: $PREFIX/bin is not in your PATH."
    echo "Add it with:"
    echo "  export PATH=\"$PREFIX/bin:\$PATH\""
    echo ""
fi
