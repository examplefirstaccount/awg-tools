#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="$HOME/.local/bin"
LIBEXEC_DIR="$HOME/.local/libexec/awgproxy"
SUDOERS_FILE="/etc/sudoers.d/awgproxy-relay"

rm -f "$BIN_DIR/awgproxy" "$BIN_DIR/awg-fetch"
rm -rf "$LIBEXEC_DIR"
echo "Removed $BIN_DIR/{awgproxy,awg-fetch} and $LIBEXEC_DIR"

if [[ -f "$SUDOERS_FILE" ]]; then
    sudo rm -f "$SUDOERS_FILE"
    echo "Removed $SUDOERS_FILE"
fi

echo "Note: ~/.config/awgproxy (fetched configs + backups) was left in place."
