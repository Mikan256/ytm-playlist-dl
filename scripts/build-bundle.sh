#!/usr/bin/env bash
set -euo pipefail

# Build a portable ytm-playlist-dl release with yt-dlp and ffmpeg included.
#
# Examples:
#   scripts/build-bundle.sh
#   scripts/build-bundle.sh --os windows --arch amd64
#   scripts/build-bundle.sh --os linux --arch arm64
#   scripts/build-bundle.sh --yt-dlp ./yt-dlp --ffmpeg ./ffmpeg --ffprobe ./ffprobe

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OS="$(go env GOOS)"
ARCH="$(go env GOARCH)"
OUT=""
VERSION="${VERSION:-0.5.0}"
YTDLP_PATH=""
FFMPEG_PATH=""
FFPROBE_PATH=""
DESKTOP=0

usage() {
  cat <<'USAGE'
Usage: scripts/build-bundle.sh [options]

Options:
  --os OS              linux, darwin or windows (default: go env GOOS)
  --arch ARCH          amd64 or arm64 (default: go env GOARCH)
  --out DIR            output directory (default: dist/ytm-playlist-dl-OS-ARCH)
  --version VERSION    version embedded in the binaries (default: 0.5.0)
  --yt-dlp PATH        use a local yt-dlp instead of downloading
  --ffmpeg PATH        use a local ffmpeg instead of downloading
  --ffprobe PATH       use a local ffprobe instead of downloading
  --desktop            also build the Wails v3 desktop application
  -h, --help           show this help
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --os) OS="$2"; shift 2 ;;
    --arch) ARCH="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --yt-dlp) YTDLP_PATH="$2"; shift 2 ;;
    --ffmpeg) FFMPEG_PATH="$2"; shift 2 ;;
    --ffprobe) FFPROBE_PATH="$2"; shift 2 ;;
    --desktop) DESKTOP=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

case "$OS/$ARCH" in
  linux/amd64|linux/arm64|darwin/amd64|darwin/arm64|windows/amd64) ;;
  *) echo "Unsupported target: $OS/$ARCH" >&2; exit 2 ;;
esac

SUFFIX=""
if [[ "$OS" == "windows" ]]; then
  SUFFIX=".exe"
fi
if [[ -z "$OUT" ]]; then
  OUT="$ROOT/dist/ytm-playlist-dl-$OS-$ARCH"
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

rm -rf "$OUT"
mkdir -p "$OUT/bin" "$OUT/LICENSES"

download() {
  local url="$1" dest="$2"
  echo "Downloading $url"
  curl -fL --retry 3 --connect-timeout 20 "$url" -o "$dest"
}

copy_executable() {
  local source="$1" dest="$2"
  cp "$source" "$dest"
  chmod +x "$dest"
}

# --- yt-dlp ---------------------------------------------------------------
if [[ -n "$YTDLP_PATH" ]]; then
  copy_executable "$YTDLP_PATH" "$OUT/bin/yt-dlp$SUFFIX"
else
  case "$OS/$ARCH" in
    linux/amd64) asset="yt-dlp_linux" ;;
    linux/arm64) asset="yt-dlp_linux_aarch64" ;;
    darwin/amd64|darwin/arm64) asset="yt-dlp_macos" ;;
    windows/amd64) asset="yt-dlp.exe" ;;
  esac
  download "https://github.com/yt-dlp/yt-dlp/releases/latest/download/$asset" \
    "$OUT/bin/yt-dlp$SUFFIX"
  chmod +x "$OUT/bin/yt-dlp$SUFFIX"
fi

# --- ffmpeg / ffprobe -----------------------------------------------------
if [[ -n "$FFMPEG_PATH" ]]; then
  copy_executable "$FFMPEG_PATH" "$OUT/bin/ffmpeg$SUFFIX"
  if [[ -n "$FFPROBE_PATH" ]]; then
    copy_executable "$FFPROBE_PATH" "$OUT/bin/ffprobe$SUFFIX"
  else
    sibling="$(dirname "$FFMPEG_PATH")/ffprobe$SUFFIX"
    if [[ ! -f "$sibling" ]]; then
      echo "ffprobe not found next to $FFMPEG_PATH; pass --ffprobe" >&2
      exit 1
    fi
    copy_executable "$sibling" "$OUT/bin/ffprobe$SUFFIX"
  fi
else
  case "$OS/$ARCH" in
    linux/amd64)
      archive="$TMP/ffmpeg.tar.xz"
      download "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz" "$archive"
      mkdir "$TMP/ffmpeg"
      tar -xJf "$archive" -C "$TMP/ffmpeg"
      find "$TMP/ffmpeg" -type f -name ffmpeg -exec cp {} "$OUT/bin/ffmpeg" \;
      find "$TMP/ffmpeg" -type f -name ffprobe -exec cp {} "$OUT/bin/ffprobe" \;
      find "$TMP/ffmpeg" -type f \( -name 'GPLv3.txt' -o -name 'COPYING*' \) \
        -exec cp {} "$OUT/LICENSES/" \; || true
      ;;
    linux/arm64)
      archive="$TMP/ffmpeg.tar.xz"
      download "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz" "$archive"
      mkdir "$TMP/ffmpeg"
      tar -xJf "$archive" -C "$TMP/ffmpeg"
      find "$TMP/ffmpeg" -type f -name ffmpeg -exec cp {} "$OUT/bin/ffmpeg" \;
      find "$TMP/ffmpeg" -type f -name ffprobe -exec cp {} "$OUT/bin/ffprobe" \;
      find "$TMP/ffmpeg" -type f \( -name 'GPLv3.txt' -o -name 'COPYING*' \) \
        -exec cp {} "$OUT/LICENSES/" \; || true
      ;;
    darwin/amd64|darwin/arm64)
      if [[ "$ARCH" == "arm64" ]]; then
        base="darwin-arm64"
      else
        base="darwin-x64"
      fi
      download "https://github.com/eugeneware/ffmpeg-static/releases/latest/download/ffmpeg-$base.gz" \
        "$TMP/ffmpeg.gz"
      download "https://github.com/eugeneware/ffmpeg-static/releases/latest/download/ffprobe-$base.gz" \
        "$TMP/ffprobe.gz"
      gzip -dc "$TMP/ffmpeg.gz" > "$OUT/bin/ffmpeg"
      gzip -dc "$TMP/ffprobe.gz" > "$OUT/bin/ffprobe"
      ;;
    windows/amd64)
      archive="$TMP/ffmpeg.zip"
      download "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip" "$archive"
      mkdir "$TMP/ffmpeg"
      unzip -q "$archive" -d "$TMP/ffmpeg"
      find "$TMP/ffmpeg" -type f -name ffmpeg.exe -exec cp {} "$OUT/bin/ffmpeg.exe" \;
      find "$TMP/ffmpeg" -type f -name ffprobe.exe -exec cp {} "$OUT/bin/ffprobe.exe" \;
      find "$TMP/ffmpeg" -type f \( -name 'LICENSE*' -o -name 'COPYING*' \) \
        -exec cp {} "$OUT/LICENSES/" \; || true
      ;;
  esac
  chmod +x "$OUT/bin/ffmpeg$SUFFIX" "$OUT/bin/ffprobe$SUFFIX"
fi

# --- Go binaries ----------------------------------------------------------
echo "Building ytm-playlist-dl $VERSION for $OS/$ARCH"
CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath \
  -ldflags "-s -w -X github.com/Mikan256/ytm-playlist-dl/internal/cli.Version=$VERSION" \
  -o "$OUT/ytm-playlist-dl$SUFFIX" \
  "$ROOT/cmd/ytm-playlist-dl"
CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath \
  -ldflags "-s -w -X github.com/Mikan256/ytm-playlist-dl/internal/cli.Version=$VERSION" \
  -o "$OUT/ytm-playlist-dl-web$SUFFIX" \
  "$ROOT/cmd/ytm-playlist-dl-web"

if [[ "$DESKTOP" -eq 1 ]]; then
  echo "Building Wails v3 desktop application for $OS/$ARCH"
  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath \
    -tags purego,production \
    -ldflags "-s -w -X github.com/Mikan256/ytm-playlist-dl/internal/cli.Version=$VERSION" \
    -o "$OUT/ytm-playlist-dl-desktop$SUFFIX" \
    "$ROOT/cmd/ytm-playlist-dl-desktop"
fi

# --- notices and documentation -------------------------------------------
cp "$ROOT/README.md" "$ROOT/SPEC.md" "$ROOT/THIRD_PARTY_NOTICES.md" "$OUT/"
cp "$ROOT/LICENSE" "$OUT/"
cp -R "$ROOT/LICENSES/." "$OUT/LICENSES/"

chmod +x "$OUT/ytm-playlist-dl$SUFFIX" "$OUT/ytm-playlist-dl-web$SUFFIX"
if [[ "$DESKTOP" -eq 1 ]]; then
  chmod +x "$OUT/ytm-playlist-dl-desktop$SUFFIX"
fi
mkdir -p "$ROOT/dist"
OUT_PARENT="$(dirname "$OUT")"
OUT_NAME="$(basename "$OUT")"
if [[ "$OS" == "windows" ]]; then
  ARCHIVE="$ROOT/dist/ytm-playlist-dl-$VERSION-$OS-$ARCH.zip"
  rm -f "$ARCHIVE"
  (cd "$OUT_PARENT" && zip -qr "$ARCHIVE" "$OUT_NAME")
else
  ARCHIVE="$ROOT/dist/ytm-playlist-dl-$VERSION-$OS-$ARCH.tar.gz"
  rm -f "$ARCHIVE"
  tar -czf "$ARCHIVE" -C "$OUT_PARENT" "$OUT_NAME"
fi

echo
echo "Bundle: $OUT"
echo "Archive: $ARCHIVE"
du -h "$ARCHIVE"
