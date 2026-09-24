.PHONY: test tidy dev build fetch-runtime package dist macos-app

test:
	go test ./internal/...

tidy:
	go mod tidy

dev:
	wails dev

build:
	wails build

fetch-runtime:
	bash scripts/fetch-runtime.sh

package:
	bash scripts/package-sidecar.sh

# Self-contained app: ffmpeg + whisper.cpp + llama.cpp + models inside the bundle.
dist: fetch-runtime build package

macos-app:
	bash ./build_macos.sh
