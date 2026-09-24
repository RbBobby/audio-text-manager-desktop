#!/usr/bin/env bash
# One-shot production build: self-contained AudioTextManager.app + zip in dist/.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
export PATH="${HOME}/go/bin:/usr/local/go/bin:/opt/homebrew/bin:${PATH}"
cd "$ROOT"

if ! command -v wails >/dev/null 2>&1; then
  echo "wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
  echo "Then: export PATH=\"\$HOME/go/bin:\$PATH\"" >&2
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  echo "Go is required (see go.mod)." >&2
  exit 1
fi

echo "==> fetch sidecar + wails build + package"
make dist

SRC="$ROOT/build/bin/AudioTextManager.app"
BIN="$SRC/Contents/MacOS/AudioTextManager"
if [[ ! -f "$BIN" ]]; then
  echo "Build produced no executable at $BIN" >&2
  exit 1
fi
chmod +x "$BIN"

find "$SRC" -name '._*' -delete
find "$SRC" -name '.DS_Store' -delete
xattr -cr "$SRC" 2>/dev/null || true
codesign --force --deep --sign - "$SRC"

if ! codesign --verify --deep --strict "$SRC"; then
  echo "codesign --verify failed; bundle is not safe to copy" >&2
  exit 1
fi

if ! find "$SRC/Contents/Resources/sidecar" -type f \( -name ffmpeg -o -name whisper-cli -o -name llama-server \) | grep -q .; then
  echo "sidecar binaries missing under Resources/sidecar" >&2
  exit 1
fi

DEST_DIR="$ROOT/dist"
rm -rf "$DEST_DIR/AudioTextManager.app" "$DEST_DIR/AudioTextManager.zip"
mkdir -p "$DEST_DIR"
ditto "$SRC" "$DEST_DIR/AudioTextManager.app"
ditto -c -k --norsrc --keepParent "$DEST_DIR/AudioTextManager.app" "$DEST_DIR/AudioTextManager.zip"

ARCH="$(lipo -archs "$DEST_DIR/AudioTextManager.app/Contents/MacOS/AudioTextManager" 2>/dev/null || file "$BIN")"
SIZE_APP="$(du -sh "$DEST_DIR/AudioTextManager.app" | awk '{print $1}')"
SIZE_ZIP="$(du -sh "$DEST_DIR/AudioTextManager.zip" | awk '{print $1}')"

echo
echo "OK  app:  $DEST_DIR/AudioTextManager.app  ($SIZE_APP)"
echo "OK  zip:  $DEST_DIR/AudioTextManager.zip  ($SIZE_ZIP)"
echo "OK  arch: $ARCH"
echo "Open locally: open \"$DEST_DIR/AudioTextManager.app\""
echo "Send the zip (not the raw .app folder). See BUILD.md."
