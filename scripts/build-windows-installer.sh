#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUNDLE=""
VERSION="${VERSION:-0.6.0}"
OUT=""

usage() {
  cat <<'USAGE'
Usage: scripts/build-windows-installer.sh --bundle DIR [options]

Options:
  --bundle DIR       Windows bundle directory built by build-bundle.sh
  --version VERSION  installer version (default: 0.6.0)
  --out FILE         output installer path
  -h, --help         show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bundle) BUNDLE="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "$BUNDLE" ]]; then
  echo "--bundle is required" >&2
  exit 2
fi
BUNDLE="$(cd "$BUNDLE" && pwd)"
if ! command -v makensis >/dev/null 2>&1; then
  echo "makensis (NSIS) is required. Install nsis and retry." >&2
  exit 1
fi
for file in ytm-playlist-dl-desktop.exe bin/yt-dlp.exe bin/ffmpeg.exe bin/ffprobe.exe; do
  if [[ ! -f "$BUNDLE/$file" ]]; then
    echo "missing $BUNDLE/$file" >&2
    exit 1
  fi
done

if [[ -z "$OUT" ]]; then
  OUT="$(dirname "$BUNDLE")/ytm-playlist-dl-${VERSION}-windows-amd64-setup.exe"
fi
mkdir -p "$(dirname "$OUT")"
OUT="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"

echo "Building Windows installer: $OUT"
makensis -V2 \
  -DVERSION="$VERSION" \
  -DSOURCE="$BUNDLE" \
  -DOUTFILE="$OUT" \
  "$ROOT/packaging/windows/installer.nsi"

echo "Installer: $OUT"
ls -lh "$OUT"
