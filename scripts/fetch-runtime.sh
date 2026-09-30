#!/usr/bin/env bash
# Download / build sidecar binaries and models into third_party/.
# Needed once per machine (or CI) before `make dist`.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TP="$ROOT/third_party"
mkdir -p "$TP/ffmpeg" "$TP/whisper" "$TP/llama" "$TP/models" "$TP/src"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  arm64|aarch64) ARCH=arm64 ;;
  x86_64|amd64) ARCH=x64 ;;
esac

need() { command -v "$1" >/dev/null 2>&1 || { echo "need $1 on PATH"; exit 1; }; }

ensure_cmake() {
  if command -v cmake >/dev/null 2>&1; then
    return
  fi
  if command -v brew >/dev/null 2>&1; then
    echo ">> brew install cmake"
    brew install cmake
    return
  fi
  echo "need cmake on PATH (or Homebrew)"
  exit 1
}
need curl
need tar
need unzip

download() {
  local url="$1" dest="$2"
  if [[ -f "$dest" && -s "$dest" ]]; then
    return 0
  fi
  echo ">> $url"
  if ! curl -fL --retry 3 -A "audio-text-manager-desktop" -o "$dest.partial" "$url"; then
    rm -f "$dest.partial"
    echo "WARN: unable to download $url; network/DNS is unavailable. Skipping this runtime asset." >&2
    return 1
  fi
  mv "$dest.partial" "$dest"
}

github_asset() {
  local repo="$1" needle="$2"
  local python_bin=python3
  if [[ "$OS" == "mingw"* || "$OS" == "msys"* || "$OS" == "cygwin"* || "$OS" == "windows"* ]] && \
    { ! command -v "$python_bin" >/dev/null 2>&1 || ! "$python_bin" -c 'import json' >/dev/null 2>&1; }; then
    python_bin=python
  fi
  if ! curl -fsSL -A "audio-text-manager-desktop" "https://api.github.com/repos/${repo}/releases?per_page=12" | \
    "$python_bin" -c "
import json,sys,re
needle=sys.argv[1]
releases=json.load(sys.stdin)
for rel in releases:
    for a in rel.get('assets') or []:
        name=a.get('name') or ''
        url=a.get('browser_download_url') or ''
        if needle in name and url:
            print(url)
            sys.exit(0)
sys.exit(1)
" "$needle"; then
    echo "WARN: unable to query GitHub releases for $repo; network/DNS is unavailable. Skipping this runtime asset." >&2
    return 1
  fi
}

fetch_ffmpeg() {
  if [[ -x "$TP/ffmpeg/ffmpeg" || -f "$TP/ffmpeg/ffmpeg.exe" ]]; then
    echo "ffmpeg already present"
    return
  fi
  if [[ "$OS" == "darwin" ]]; then
    download "https://evermeet.cx/ffmpeg/getrelease/ffmpeg/zip" "$TP/src/ffmpeg.zip"
    download "https://evermeet.cx/ffmpeg/getrelease/ffprobe/zip" "$TP/src/ffprobe.zip"
    unzip -o "$TP/src/ffmpeg.zip" -d "$TP/src/ffmpeg-bin"
    unzip -o "$TP/src/ffprobe.zip" -d "$TP/src/ffprobe-bin"
    find "$TP/src/ffmpeg-bin" "$TP/src/ffprobe-bin" -type f -perm -u+x | while read -r f; do
      case "$(basename "$f")" in
        ffmpeg) cp "$f" "$TP/ffmpeg/ffmpeg" ;;
        ffprobe) cp "$f" "$TP/ffmpeg/ffprobe" ;;
      esac
    done
    chmod +x "$TP/ffmpeg/ffmpeg" "$TP/ffmpeg/ffprobe"
    return
  fi
  if [[ "$OS" == "mingw"* || "$OS" == "msys"* || "$OS" == "cygwin"* || "$OS" == "windows"* ]]; then
    download "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip" "$TP/src/ffmpeg-win.zip"
    rm -rf "$TP/src/ffmpeg-win"
    mkdir -p "$TP/src/ffmpeg-win"
    unzip -o "$TP/src/ffmpeg-win.zip" -d "$TP/src/ffmpeg-win"
    local ff
    ff="$(find "$TP/src/ffmpeg-win" -type f -name ffmpeg.exe | head -n1)"
    local fp
    fp="$(find "$TP/src/ffmpeg-win" -type f -name ffprobe.exe | head -n1)"
    if [[ -z "$ff" || -z "$fp" ]]; then
      echo "ffmpeg.exe / ffprobe.exe not found in essentials zip" >&2
      exit 1
    fi
    cp "$ff" "$TP/ffmpeg/ffmpeg.exe"
    cp "$fp" "$TP/ffmpeg/ffprobe.exe"
    return
  fi
  echo "unsupported OS for ffmpeg auto-fetch: $OS"
  exit 1
}

fetch_llama() {
  if [[ -x "$TP/llama/llama-server" || -f "$TP/llama/llama-server.exe" ]]; then
    echo "llama-server already present"
    return
  fi
  local needle
  if [[ "$OS" == "darwin" ]]; then
    needle="bin-macos-${ARCH}.tar.gz"
  else
    needle="bin-win-cpu-x64.zip"
  fi
  local url
  if ! url="$(github_asset "ggml-org/llama.cpp" "$needle")"; then
    echo "WARN: llama runtime could not be downloaded; continuing without llama-server." >&2
    return 0
  fi
  local archive="$TP/src/llama-bin.tgz"
  if [[ "$url" == *.zip ]]; then
    archive="$TP/src/llama-bin.zip"
  fi
  if ! download "$url" "$archive"; then
    echo "WARN: llama archive download failed; skipping llama runtime." >&2
    return 0
  fi
  rm -rf "$TP/src/llama-unpack"
  mkdir -p "$TP/src/llama-unpack"
  if [[ "$archive" == *.zip ]]; then
    unzip -o "$archive" -d "$TP/src/llama-unpack"
  else
    tar -xzf "$archive" -C "$TP/src/llama-unpack"
  fi
  local server
  server="$(find "$TP/src/llama-unpack" -type f \( -name 'llama-server' -o -name 'llama-server.exe' \) | head -n1)"
  if [[ -z "$server" ]]; then
    echo "WARN: llama-server not found in archive; skipping llama runtime." >&2
    return 0
  fi
  cp -R "$(dirname "$server")/." "$TP/llama/"
  chmod +x "$TP/llama/llama-server" 2>/dev/null || true
}

fetch_whisper() {
  if [[ -x "$TP/whisper/whisper-cli" || -f "$TP/whisper/whisper-cli.exe" ]]; then
    echo "whisper-cli already present"
    return
  fi
  if [[ "$OS" == "darwin" ]]; then
    ensure_cmake
    need git
    if [[ ! -d "$TP/src/whisper.cpp/.git" ]]; then
      git clone --depth 1 https://github.com/ggml-org/whisper.cpp "$TP/src/whisper.cpp"
    fi
    cmake -S "$TP/src/whisper.cpp" -B "$TP/src/whisper.cpp/build" \
      -DCMAKE_BUILD_TYPE=Release \
      -DBUILD_SHARED_LIBS=OFF
    cmake --build "$TP/src/whisper.cpp/build" --config Release -j --target whisper-cli
    local bin
    bin="$(find "$TP/src/whisper.cpp/build" -type f -name whisper-cli | head -n1)"
    cp "$bin" "$TP/whisper/whisper-cli"
    chmod +x "$TP/whisper/whisper-cli"
    find "$TP/src/whisper.cpp/build" -name '*.dylib' -o -name '*.metallib' -o -name '*.metal' |
      while read -r f; do cp "$f" "$TP/whisper/" || true; done
    return
  fi
  local url
  if ! url="$(github_asset "ggml-org/whisper.cpp" "whisper-bin-x64.zip")"; then
    echo "WARN: whisper runtime could not be downloaded; continuing without whisper-cli." >&2
    return 0
  fi
  if ! download "$url" "$TP/src/whisper-bin.zip"; then
    echo "WARN: whisper archive download failed; skipping whisper runtime." >&2
    return 0
  fi
  unzip -o "$TP/src/whisper-bin.zip" -d "$TP/src/whisper-unpack"
  local cli
  cli="$(find "$TP/src/whisper-unpack" -type f \( -name 'whisper-cli.exe' -o -name 'whisper-cli' -o -name 'main.exe' \) | head -n1)"
  if [[ -z "$cli" ]]; then
    echo "WARN: whisper-cli was not found in the archive; skipping whisper runtime." >&2
    return 0
  fi
  cp -R "$(dirname "$cli")/." "$TP/whisper/"
}

fetch_models() {
  for model_url in \
    "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small-q5_1.bin" \
    "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium-q5_0.bin" \
    "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-q5_0.bin" \
    "https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf"; do
    local name
    name="$(basename "$model_url")"
    if ! download "$model_url" "$TP/models/$name"; then
      echo "WARN: model $name could not be downloaded; network/DNS may be unavailable." >&2
    fi
  done
}

fetch_ffmpeg
fetch_llama
fetch_whisper
fetch_models

echo "Runtime cached in $TP"
ls -lh "$TP/ffmpeg" "$TP/whisper" "$TP/llama" "$TP/models"
