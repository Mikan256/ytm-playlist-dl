#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUNDLE=""
ARCH="amd64"
VERSION="${VERSION:-0.6.0}"
OUT=""
APPIMAGE=1

usage() {
  cat <<'USAGE'
Usage: scripts/build-linux-packages.sh --bundle DIR [options]

Options:
  --bundle DIR       Linux bundle directory built by build-bundle.sh
  --arch ARCH        amd64 or arm64 (default: amd64)
  --version VERSION  package version (default: 0.6.0)
  --out DIR          output directory (default: dist)
  --no-appimage      skip AppImage creation
  -h, --help         show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bundle) BUNDLE="$2"; shift 2 ;;
    --arch) ARCH="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    --no-appimage) APPIMAGE=0; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "$BUNDLE" ]]; then
  echo "--bundle is required" >&2
  exit 2
fi
BUNDLE="$(cd "$BUNDLE" && pwd)"
if [[ -z "$OUT" ]]; then OUT="$ROOT/dist"; fi
mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd)"

for file in ytm-playlist-dl-desktop bin/yt-dlp bin/ffmpeg bin/ffprobe; do
  if [[ ! -f "$BUNDLE/$file" ]]; then
    echo "missing $BUNDLE/$file" >&2
    exit 1
  fi
done

NFPM=(nfpm)
if ! command -v nfpm >/dev/null 2>&1; then
  NFPM=(go run github.com/goreleaser/nfpm/v2/cmd/nfpm@latest)
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cat > "$TMP/nfpm.yaml" <<YAML
name: ytm-playlist-dl
arch: ${ARCH}
platform: linux
version: ${VERSION}
section: sound
priority: optional
maintainer: Mikan256
description: Download YouTube Music playlists and videos
license: MIT
contents:
  - src: ${BUNDLE}/ytm-playlist-dl-desktop
    dst: /opt/ytm-playlist-dl/ytm-playlist-dl-desktop
  - src: ${BUNDLE}/ytm-playlist-dl
    dst: /usr/bin/ytm-playlist-dl
  - src: ${BUNDLE}/ytm-playlist-dl-web
    dst: /usr/bin/ytm-playlist-dl-web
  - src: ${BUNDLE}/bin/yt-dlp
    dst: /opt/ytm-playlist-dl/bin/yt-dlp
  - src: ${BUNDLE}/bin/ffmpeg
    dst: /opt/ytm-playlist-dl/bin/ffmpeg
  - src: ${BUNDLE}/bin/ffprobe
    dst: /opt/ytm-playlist-dl/bin/ffprobe
  - src: ${ROOT}/packaging/linux/ytm-playlist-dl.desktop
    dst: /usr/share/applications/ytm-playlist-dl.desktop
  - src: ${ROOT}/cmd/ytm-playlist-dl-desktop/assets/icon.png
    dst: /opt/ytm-playlist-dl/icon.png
  - src: ${ROOT}/cmd/ytm-playlist-dl-desktop/assets/icon.png
    dst: /usr/share/icons/hicolor/64x64/apps/ytm-playlist-dl.png
  - src: ${BUNDLE}/LICENSE
    dst: /usr/share/doc/ytm-playlist-dl/LICENSE
  - src: ${BUNDLE}/LICENSES
    dst: /usr/share/doc/ytm-playlist-dl/LICENSES
    type: tree
YAML

for packager in deb rpm; do
  target="$OUT/ytm-playlist-dl-${VERSION}-linux-${ARCH}.${packager}"
  echo "Building $packager: $target"
  "${NFPM[@]}" package --packager "$packager" --config "$TMP/nfpm.yaml" --target "$target"
done

if [[ "$APPIMAGE" -eq 1 && "$ARCH" == "amd64" ]]; then
  APPDIR="$TMP/AppDir"
  mkdir -p "$APPDIR/usr/bin/bin" "$APPDIR/usr/share/applications" \
    "$APPDIR/usr/share/icons/hicolor/64x64/apps" "$APPDIR/usr/share/doc/ytm-playlist-dl"
  cp "$BUNDLE/ytm-playlist-dl-desktop" "$APPDIR/usr/bin/ytm-playlist-dl-desktop"
  cp "$BUNDLE/ytm-playlist-dl" "$APPDIR/usr/bin/ytm-playlist-dl"
  cp "$BUNDLE/ytm-playlist-dl-web" "$APPDIR/usr/bin/ytm-playlist-dl-web"
  cp "$BUNDLE/bin/"* "$APPDIR/usr/bin/bin/"
  cp "$ROOT/packaging/linux/ytm-playlist-dl.desktop" "$APPDIR/ytm-playlist-dl.desktop"
  cp "$ROOT/packaging/linux/ytm-playlist-dl.desktop" "$APPDIR/usr/share/applications/"
  cp "$ROOT/cmd/ytm-playlist-dl-desktop/assets/icon.png" "$APPDIR/ytm-playlist-dl.png"
  cp "$ROOT/cmd/ytm-playlist-dl-desktop/assets/icon.png" \
    "$APPDIR/usr/share/icons/hicolor/64x64/apps/ytm-playlist-dl.png"
  cp "$BUNDLE/LICENSE" "$APPDIR/usr/share/doc/ytm-playlist-dl/"
  cp -R "$BUNDLE/LICENSES" "$APPDIR/usr/share/doc/ytm-playlist-dl/"
  ln -s usr/bin/ytm-playlist-dl-desktop "$APPDIR/AppRun"

  TOOL="$TMP/appimagetool.AppImage"
  if command -v appimagetool >/dev/null 2>&1; then
    APPIMAGETOOL=(appimagetool)
  else
    echo "Downloading appimagetool..."
    curl -fL --retry 3 \
      "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage" \
      -o "$TOOL"
    chmod +x "$TOOL"
    APPIMAGETOOL=("$TOOL" --appimage-extract-and-run)
  fi
  TARGET="$OUT/ytm-playlist-dl-${VERSION}-linux-${ARCH}.AppImage"
  if ! ARCH=x86_64 "${APPIMAGETOOL[@]}" "$APPDIR" "$TARGET"; then
    echo "Warning: AppImage build failed; deb/rpm packages are still available." >&2
    rm -f "$TARGET"
  fi
fi

echo
ls -lh "$OUT"/ytm-playlist-dl-"${VERSION}"-linux-"${ARCH}".* 2>/dev/null || true
