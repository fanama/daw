APP := gorecord
BIN := bin/$(APP)
ADDR ?= 127.0.0.1:8080
BUNDLE := dist/$(APP).app
SITE := dist-site

.PHONY: build bundle run test vet fmt caps clean web serve site

build: web
	go build -o $(BIN) ./cmd/gorecord

bundle: build
	rm -rf "$(BUNDLE)"
	mkdir -p "$(BUNDLE)/Contents/MacOS" "$(BUNDLE)/Contents/Resources/web"
	cp "$(BIN)" "$(BUNDLE)/Contents/MacOS/$(APP)"
	cp -R web/. "$(BUNDLE)/Contents/Resources/web/"
	cp packaging/macos/Info.plist "$(BUNDLE)/Contents/Info.plist"
	printf 'APPL????' > "$(BUNDLE)/Contents/PkgInfo"
	@echo "built $(BUNDLE)"

run: web
	go run ./cmd/gorecord -addr $(ADDR)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

caps:
	./scripts/check-caps.sh

web:
	GOOS=js GOARCH=wasm go build -o web/gorecord.wasm ./webapp
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js

serve: web
	go run ./cmd/gorecord -window=none -addr $(ADDR)

site:
	node scripts/build-site.mjs

clean:
	rm -rf bin dist $(SITE) web/gorecord.wasm web/wasm_exec.js
