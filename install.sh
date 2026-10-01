#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BIN_DIR="$HOME/.local/bin"
LIBEXEC_DIR="$HOME/.local/libexec/awgproxy"
CONFIG_ROOT="$HOME/.config/awgproxy"
SUDOERS_FILE="/etc/sudoers.d/awgproxy-relay"

die() { echo "Error: $*" >&2; exit 1; }

[[ "$(uname -s)" == "Darwin" ]] || die "This toolkit only works on macOS (uses /dev/bpf)."

echo "== Checking prerequisites =="

command -v python3 >/dev/null 2>&1 || die "python3 not found (needed for awg-fetch). Install via pyenv or Homebrew."
echo "  python3: $(command -v python3) ($(python3 --version 2>&1))"

command -v go >/dev/null 2>&1 || die "go not found (needed to build the relay). Install with: brew install go"
echo "  go: $(go version)"

brew --prefix libpcap >/dev/null 2>&1 || die "libpcap not found (needed to build the relay). Install with: brew install libpcap"
echo "  libpcap: $(brew --prefix libpcap)"

if ! command -v wireproxy >/dev/null 2>&1 && [[ ! -x "$BIN_DIR/wireproxy" ]]; then
    cat >&2 <<EOF

wireproxy not found. It's not in Homebrew — grab a release binary for your
CPU arch from:
    https://github.com/pufferffish/wireproxy/releases
(or, for AmneziaWG configs specifically, a fork such as
artem-russkikh/wireproxy-awg) and place it at $BIN_DIR/wireproxy
(chmod +x), then re-run this script.
EOF
    die "missing dependency: wireproxy"
fi
echo "  wireproxy: $(command -v wireproxy || echo "$BIN_DIR/wireproxy")"

echo
echo "== Building the relay =="
"$REPO_DIR/awgproxy/relay/build.sh"

echo
echo "== Installing =="
mkdir -p "$BIN_DIR" "$LIBEXEC_DIR" "$CONFIG_ROOT/configs" "$CONFIG_ROOT/backups"

install -m 755 "$REPO_DIR/awgproxy/awgproxy" "$BIN_DIR/awgproxy"
install -m 755 "$REPO_DIR/awg-fetch/awg-fetch" "$BIN_DIR/awg-fetch"
install -m 755 "$REPO_DIR/awgproxy/bin/relay" "$LIBEXEC_DIR/relay"

echo "  $BIN_DIR/awgproxy"
echo "  $BIN_DIR/awg-fetch"
echo "  $LIBEXEC_DIR/relay"
echo "  $CONFIG_ROOT/{configs,backups}"

case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo "  NOTE: $BIN_DIR is not on your PATH — add 'export PATH=\"$BIN_DIR:\$PATH\"' to your shell rc" ;;
esac

echo
echo "== Setting up sudoers rule for awgproxy --bypass =="
echo "  (needs your sudo password once; the relay itself will then run without prompting)"

SUDOERS_LINE="$(whoami) ALL=(root) NOPASSWD: $LIBEXEC_DIR/relay *"

TMP_SUDOERS=$(mktemp)
echo "$SUDOERS_LINE" > "$TMP_SUDOERS"

if ! sudo visudo -cf "$TMP_SUDOERS" >/dev/null 2>&1; then
    rm -f "$TMP_SUDOERS"
    die "generated sudoers rule failed validation, aborting"
fi

sudo install -m 440 -o root -g wheel "$TMP_SUDOERS" "$SUDOERS_FILE"
rm -f "$TMP_SUDOERS"
echo "  wrote $SUDOERS_FILE:"
echo "    $SUDOERS_LINE"

cat <<EOF

== Done ==
Installed:
  $BIN_DIR/awgproxy, $BIN_DIR/awg-fetch
  $LIBEXEC_DIR/relay
  $CONFIG_ROOT/{configs,backups}
EOF
