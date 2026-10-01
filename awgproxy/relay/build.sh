#!/usr/bin/env bash
# Builds bin/relay, statically linking libpcap so the resulting binary has
# no runtime dependency on Homebrew (only needed at build time).
set -euo pipefail

RELAY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$(cd "$RELAY_DIR/.." && mkdir -p bin && cd bin && pwd)"
OUT="$BIN_DIR/relay"

die() { echo "Error: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || die "go not found. Install with: brew install go"

LIBPCAP_PREFIX="$(brew --prefix libpcap 2>/dev/null)" || die "libpcap not found. Install with: brew install libpcap"
[[ -f "$LIBPCAP_PREFIX/lib/libpcap.a" ]] || die "libpcap.a not found under $LIBPCAP_PREFIX/lib"

STATIC_DIR="$(mktemp -d)"
trap 'rm -rf "$STATIC_DIR"' EXIT
cp "$LIBPCAP_PREFIX/lib/libpcap.a" "$STATIC_DIR/"

cd "$RELAY_DIR"
CGO_ENABLED=1 \
CGO_CFLAGS="-I$LIBPCAP_PREFIX/include" \
CGO_LDFLAGS="-L$STATIC_DIR" \
    go build -o "$OUT" .

echo "Built $OUT"
