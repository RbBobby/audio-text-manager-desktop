#!/usr/bin/env bash
# Production Windows builds. Run in Git Bash on a Windows PC (not from macOS).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
export PATH="${HOME}/go/bin:/usr/local/go/bin:${PATH}"
cd "$ROOT"

uname_s="$(uname -s | tr '[:upper:]' '[:lower:]')"
if [[ "$uname_s" != *mingw* && "$uname_s" != *msys* && "$uname_s" != *cygwin* && "$uname_s" != *windows* ]]; then
  echo "This script must run on Windows (Git Bash). Wails cannot cross-compile WebView from macOS." >&2
  exit 1
fi

FLAVORS=()
case "${1:-all}" in
  all|"") FLAVORS=(medium speakers) ;;
  medium|speakers) FLAVORS=("$1") ;;
  *)
    echo "usage: $0 [all|medium|speakers]" >&2
    exit 1
    ;;
esac

if ! command -v wails >/dev/null 2>&1; then
  echo "wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
  echo "Then add %USERPROFILE%\\go\\bin to PATH." >&2
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Go is required (see go.mod)." >&2
  exit 1
fi

echo "==> fetch sidecar (ffmpeg, whisper, llama, models)"
bash scripts/fetch-runtime.sh
echo "==> wails build"
wails build

EXE="$ROOT/build/bin/AudioTextManager.exe"
if [[ ! -f "$EXE" ]]; then
  echo "Build produced no executable at $EXE" >&2
  exit 1
fi

zip_dir() {
  local src="$1" zip="$2"
  rm -f "$zip"
  if command -v zip >/dev/null 2>&1; then
    (cd "$(dirname "$src")" && zip -r -q "$zip" "$(basename "$src")")
    return
  fi
  powershell.exe -NoProfile -Command "Compress-Archive -Path '$src' -DestinationPath '$zip' -Force"
}

stage() {
  local flavor="$1"
  echo "==> package $flavor"
  FLAVOR="$flavor" bash scripts/package-sidecar.sh

  local dest="$ROOT/dist/$flavor/AudioTextManager"
  rm -rf "$dest"
  mkdir -p "$dest"
  cp "$EXE" "$dest/"
  cp -R "$ROOT/build/bin/sidecar" "$dest/"
  cp -R "$ROOT/build/bin/models" "$dest/"
  zip_dir "$dest" "$ROOT/dist/$flavor/AudioTextManager-windows.zip"
  echo "OK  $dest"
  echo "OK  $ROOT/dist/$flavor/AudioTextManager-windows.zip"
}

for f in "${FLAVORS[@]}"; do
  stage "$f"
done

echo
echo "Give the user the folder AudioTextManager or the zip. They run AudioTextManager.exe"
echo "WebView2 must be installed on the target PC."
