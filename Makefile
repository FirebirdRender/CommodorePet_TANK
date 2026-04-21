.PHONY: wasm wasm-size serve dev copy-font copy-wasm-exec copy-web-fonts clean server bot build-all test test-e2e test-wasm test-headless test-all govulncheck

copy-font:
	@true  # font embedded via internal/assets/fonts/PetMe64.ttf — no copy needed

copy-web-fonts:
	@mkdir -p web/fonts
	@cp internal/assets/fonts/PetMe64.ttf web/fonts/PetMe.ttf

copy-wasm-exec:
	@cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js

wasm: copy-font copy-wasm-exec copy-web-fonts
	GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w" -o web/game.wasm ./cmd/client/

wasm-size: wasm
	@echo "Raw size:   $$(du -h web/game.wasm | cut -f1)"
	@echo "Gzip size:  $$(gzip -c web/game.wasm | wc -c | awk '{printf "%.1f MB\n", $$1/1048576}')"
	@gzip -c web/game.wasm | wc -c | awk '{exit !($$1 < 5*1048576)}' && echo "PASS: gzipped < 5MB" || echo "FAIL: gzipped >= 5MB"

server:
	@mkdir -p bin
	go build -o bin/tank-server ./cmd/server/

bot:
	@mkdir -p bin
	go build -o bin/tank-bot ./cmd/bot-go/

skilltest:
	@mkdir -p bin
	go build -o bin/bot-skilltest ./cmd/bot-skilltest/

build-all: server bot wasm skilltest
	@echo "Build complete: bin/tank-server + bin/tank-bot + bin/bot-skilltest + web/game.wasm"

serve:
	cd web && python3 -m http.server 8081

dev: wasm copy-wasm-exec
	go run ./cmd/server/ -addr :8080 -dir web

test:
	go test -race ./... -count=1

test-headless:
	go test -race ./client/... -count=1 -run "TestNewGameState|TestApply|TestReset|TestKeyToDir|TestCellGlyphs|TestBarrel|TestGlyphCache|TestNewRenderer|TestColor|TestDimension|TestNewGameSetsPlayerName|TestExportGameStateNativeNoop|TestAnimTick"

test-wasm:
	@echo "==> WASM compile verification (no runtime tests — requires browser)"
	GOOS=js GOARCH=wasm go build ./client/...
	GOOS=js GOARCH=wasm go vet ./client/...
	@echo "WASM compile check passed."

test-e2e: build-all
	cd test/e2e && npm install && npx playwright install chromium && npx playwright test

test-all: test test-headless test-e2e
	@echo "All test suites complete."

govulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...

clean:
	rm -f web/game.wasm
	rm -rf bin/
	rm -rf web/fonts/

lagproxy:
	go run ./cmd/lagproxy/ -listen :8081 -target :8080 -latency 75 -jitter 10
