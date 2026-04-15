.PHONY: wasm wasm-size serve dev copy-font copy-wasm-exec copy-web-fonts clean server build-all test test-e2e test-wasm test-headless test-all

copy-font:
	@mkdir -p internal/assets/fonts
	@cp tank_game/assets/fonts/PetMe64.ttf internal/assets/fonts/

copy-web-fonts:
	@mkdir -p web/fonts
	@cp docs/FONTS/PetMe.ttf web/fonts/PetMe.ttf

copy-wasm-exec:
	@cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js

wasm: copy-font copy-wasm-exec copy-web-fonts
	GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w" -o web/game.wasm ./cmd/client/

wasm-size: wasm
	@echo "Raw size:   $$(du -h web/game.wasm | cut -f1)"
	@echo "Gzip size:  $$(gzip -c web/game.wasm | wc -c | awk '{printf "%.1f MB\n", $$1/1048576}')"
	@gzip -c web/game.wasm | wc -c | awk '{exit !($$1 < 5*1048576)}' && echo "PASS: gzipped < 5MB" || echo "FAIL: gzipped >= 5MB"

server:
	go build -o bin/tank-server ./cmd/server/

build-all: server wasm
	@echo "Build complete: bin/tank-server + web/game.wasm"

serve:
	cd web && python3 -m http.server 8081

dev: wasm copy-wasm-exec
	go run ./cmd/server/ -addr :8080 -dir web

test:
	go test -race ./... -count=1

test-headless:
	go test -race ./client/... -count=1 -run "TestNewGameState|TestApply|TestReset|TestKeyToDir|TestCellGlyphs|TestBarrel|TestGlyphCache|TestNewRenderer|TestColor|TestDimension|TestNewGameSetsPlayerName|TestExportGameStateNativeNoop|TestAnimTick"

test-e2e: build-all
	cd test/e2e && npm install && npx playwright install chromium && npx playwright test

test-all: test test-headless test-e2e
	@echo "All test suites complete."

clean:
	rm -f web/game.wasm
	rm -rf bin/
	rm -rf web/fonts/

lagproxy:
	go run ./cmd/lagproxy/ -listen :8081 -target :8080 -latency 75 -jitter 10
