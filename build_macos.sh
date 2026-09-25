#!/usr/bin/env bash
# Production macOS builds: medium (small+medium) and/or speakers (small+large-v3).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
export PATH="${HOME}/go/bin:/usr/local/go/bin:/opt/homebrew/bin:${PATH}"
cd "$ROOT"

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
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Go is required (see go.mod)." >&2
  exit 1
fi

echo "==> fetch sidecar"
bash scripts/fetch-runtime.sh
echo "==> wails build"
wails build

SRC="$ROOT/build/bin/AudioTextManager.app"
BIN="$SRC/Contents/MacOS/AudioTextManager"
if [[ ! -f "$BIN" ]]; then
  echo "Build produced no executable at $BIN" >&2
  exit 1
fi

stage() {
  local flavor="$1"
  echo "==> package $flavor"
  FLAVOR="$flavor" bash scripts/package-sidecar.sh
  chmod +x "$BIN"
  find "$SRC" -name '._*' -delete
  find "$SRC" -name '.DS_Store' -delete
  xattr -cr "$SRC" 2>/dev/null || true
  codesign --force --deep --sign - "$SRC"
  codesign --verify --deep --strict "$SRC"

  local dest="$ROOT/dist/$flavor"
  rm -rf "$dest"
  mkdir -p "$dest"
  ditto "$SRC" "$dest/AudioTextManager.app"
  ditto -c -k --norsrc --keepParent "$dest/AudioTextManager.app" "$dest/AudioTextManager.zip"
  echo "OK  $dest/AudioTextManager.app  ($(du -sh "$dest/AudioTextManager.app" | awk '{print $1}'))"
  echo "OK  $dest/AudioTextManager.zip"
}

for f in "${FLAVORS[@]}"; do
  stage "$f"
done

echo
echo "Send the zip from dist/<flavor>/ (not the raw .app folder). See BUILD.md."
