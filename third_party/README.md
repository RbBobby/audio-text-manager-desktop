# Sidecar binaries (not committed)

`make fetch-runtime` fills this tree. Packaging copies a **subset** of Whisper weights:

- `medium` build: `ggml-small-q5_1.bin` + `ggml-medium-q5_0.bin`
- `speakers` build: `ggml-small-q5_1.bin` + `ggml-large-v3-q5_0.bin` (+ `flavor.json`)

- `ffmpeg/` — `ffmpeg` and `ffprobe`
- `whisper/` — `whisper-cli` (and any dylibs / Metal shaders)
- `llama/` — `llama-server` from llama.cpp
- `models/` — `ggml-small-q5_1.bin`, `ggml-medium-q5_0.bin`, `ggml-large-v3-q5_0.bin`, `qwen2.5-3b-instruct-q4_k_m.gguf`

At runtime the app looks in:

- `AudioTextManager.app/Contents/Resources/sidecar/{ffmpeg,whisper,llama}`
- `AudioTextManager.app/Contents/Resources/models`
- `third_party/...` (for `wails dev`)
- settings / env `ATM_FFMPEG_BIN`, `ATM_FFPROBE_BIN`, `ATM_WHISPER_BIN`, `ATM_LLAMA_BIN`, `ATM_LLAMA_MODEL`
- `PATH` as a last resort

If a model is missing from the bundle, it is downloaded into the per-user models directory on first use.
