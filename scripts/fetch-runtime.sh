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
  curl -fL --retry 3 -A "audio-text-manager-desktop" -o "$dest.partial" "$url"
  mv "$dest.partial" "$dest"
}

github_asset() {
  local repo="$1" needle="$2"
  curl -fsSL -A "audio-text-manager-desktop" "https://api.github.com/repos/${repo}/releases?per_page=12" |
    python3 -c "
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
" "$needle"
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
    echo "On Windows download ffmpeg essentials into third_party/ffmpeg (ffmpeg.exe, ffprobe.exe)"
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
  url="$(github_asset "ggml-org/llama.cpp" "$needle")"
  local archive="$TP/src/llama-bin.tgz"
  if [[ "$url" == *.zip ]]; then
    archive="$TP/src/llama-bin.zip"
  fi
  download "$url" "$archive"
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
    echo "llama-server not found in archive"
    exit 1
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
  url="$(github_asset "ggml-org/whisper.cpp" "whisper-bin-x64.zip")"
  download "$url" "$TP/src/whisper-bin.zip"
  unzip -o "$TP/src/whisper-bin.zip" -d "$TP/src/whisper-unpack"
  local cli
  cli="$(find "$TP/src/whisper-unpack" -type f \( -name 'whisper-cli.exe' -o -name 'whisper-cli' -o -name 'main.exe' \) | head -n1)"
  cp -R "$(dirname "$cli")/." "$TP/whisper/"
}

fetch_models() {
  download "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small-q5_1.bin" "$TP/models/ggml-small-q5_1.bin"
  download "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium-q5_0.bin" "$TP/models/ggml-medium-q5_0.bin"
  download "https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf" \
    "$TP/models/qwen2.5-3b-instruct-q4_k_m.gguf"
}

fetch_ffmpeg
fetch_llama
fetch_whisper
fetch_models

echo "Runtime cached in $TP"
ls -lh "$TP/ffmpeg" "$TP/whisper" "$TP/llama" "$TP/models"
