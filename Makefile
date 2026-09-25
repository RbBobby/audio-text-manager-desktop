.PHONY: test tidy dev build fetch-runtime package dist macos-app windows-app

FLAVOR ?= medium

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
	FLAVOR=$(FLAVOR) bash scripts/package-sidecar.sh

# One flavor into build/bin/. For both zips use ./build_macos.sh or ./build_windows.sh
dist: fetch-runtime build package

macos-app:
	bash ./build_macos.sh

windows-app:
	bash ./build_windows.sh
