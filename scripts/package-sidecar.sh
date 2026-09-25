#!/usr/bin/env bash
# Copy third_party sidecars/models into the Wails .app / .exe output.
# Dereferences symlinks and strips AppleDouble so a copied .app stays codesign-valid.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TP="$ROOT/third_party"
APP="$ROOT/build/bin/AudioTextManager.app"
WIN="$ROOT/build/bin/AudioTextManager.exe"
FLAVOR="${FLAVOR:-}"

if [[ "$FLAVOR" != "medium" && "$FLAVOR" != "speakers" ]]; then
  echo "Set FLAVOR=medium (small+medium) or FLAVOR=speakers (small+large-v3 + diarize)" >&2
  exit 1
fi

if [[ "$FLAVOR" == "speakers" ]]; then
  ASR_MODELS=(ggml-small-q5_1.bin ggml-large-v3-q5_0.bin)
  FLAVOR_JSON='{"id":"speakers","speakers":true,"presets":["fast","large"]}'
else
  ASR_MODELS=(ggml-small-q5_1.bin ggml-medium-q5_0.bin)
  FLAVOR_JSON='{"id":"medium","speakers":false,"presets":["fast","medium"]}'
fi

copy_flavor_models() {
  local dest="$1"
  rm -rf "$dest"
  mkdir -p "$dest"
  local f
  for f in "${ASR_MODELS[@]}" qwen2.5-3b-instruct-q4_k_m.gguf; do
    if [[ ! -s "$TP/models/$f" ]]; then
      echo "missing model $TP/models/$f — run make fetch-runtime" >&2
      exit 1
    fi
    cp "$TP/models/$f" "$dest/$f"
  done
  printf '%s\n' "$FLAVOR_JSON" > "$dest/flavor.json"
}

copy_tree() {
  local src="$1" dest="$2"
  mkdir -p "$dest"
  if [[ -d "$src" ]]; then
    cp -R "$src"/. "$dest/"
  fi
}

# Follow symlinks so llama dylibs are real files (cloud/zip often drop links).
copy_tree_deref() {
  local src="$1" dest="$2"
  rm -rf "$dest"
  mkdir -p "$dest"
  if [[ ! -d "$src" ]]; then
    return 0
  fi
  if cp -R -L "$src"/. "$dest/" 2>/dev/null; then
    return 0
  fi
  copy_tree "$src" "$dest"
  while IFS= read -r -d '' link; do
    if [[ ! -e "$link" ]]; then
      rm -f "$link"
      continue
    fi
    tmp="${link}.atm-real"
    rm -rf "$tmp"
    cp -R "$link" "$tmp"
    rm -f "$link"
    mv "$tmp" "$link"
  done < <(find "$dest" -type l -print0)
}

sanitize_bundle() {
  local root="$1"
  find "$root" -name '._*' -delete
  find "$root" -name '.DS_Store' -delete
  if command -v xattr >/dev/null 2>&1; then
    xattr -cr "$root" 2>/dev/null || true
  fi
}

if [[ -d "$APP" ]]; then
  RES="$APP/Contents/Resources"
  copy_tree_deref "$TP/ffmpeg" "$RES/sidecar/ffmpeg"
  copy_tree_deref "$TP/whisper" "$RES/sidecar/whisper"
  copy_tree_deref "$TP/llama" "$RES/sidecar/llama"
  copy_flavor_models "$RES/models"
  chmod -R u+w "$RES/sidecar" "$RES/models" || true
  find "$RES/sidecar" -type f \( \
    -name 'ffmpeg' -o -name 'ffprobe' -o -name 'whisper-cli' \
    -o -name 'llama-server' -o -name 'llama-cli' \
  \) -exec chmod +x {} \;
  if [[ -f "$APP/Contents/MacOS/AudioTextManager" ]]; then
    chmod +x "$APP/Contents/MacOS/AudioTextManager"
  fi
  sanitize_bundle "$APP"
  if command -v codesign >/dev/null 2>&1; then
    codesign --force --deep --sign - "$APP"
  fi
  echo "Packaged $FLAVOR into $APP"
  du -sh "$APP"
  exit 0
fi

if [[ -f "$WIN" ]]; then
  DIR="$(dirname "$WIN")"
  copy_tree "$TP/ffmpeg" "$DIR/sidecar/ffmpeg"
  copy_tree "$TP/whisper" "$DIR/sidecar/whisper"
  copy_tree "$TP/llama" "$DIR/sidecar/llama"
  copy_flavor_models "$DIR/models"
  echo "Packaged $FLAVOR next to $WIN"
  exit 0
fi

echo "No Wails output at $APP or $WIN — run wails build first"
exit 1
