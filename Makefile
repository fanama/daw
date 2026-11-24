APP := fanamix
BIN := bin/$(APP)
ADDR ?= 127.0.0.1:8080
BUNDLE := dist/$(APP).app
SITE := dist-site

.PHONY: build bundle run test vet fmt caps clean web serve site

build: web
	go build -o $(BIN) ./cmd/fanamix

bundle: build
	rm -rf "$(BUNDLE)"
	mkdir -p "$(BUNDLE)/Contents/MacOS" "$(BUNDLE)/Contents/Resources/web"
	cp "$(BIN)" "$(BUNDLE)/Contents/MacOS/$(APP)"
	cp -R web/. "$(BUNDLE)/Contents/Resources/web/"
	cp packaging/macos/Info.plist "$(BUNDLE)/Contents/Info.plist"
	printf 'APPL????' > "$(BUNDLE)/Contents/PkgInfo"
	@echo "built $(BUNDLE)"

run: web
	go run ./cmd/fanamix -addr $(ADDR)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

caps:
	./scripts/check-caps.sh

web:
	GOOS=js GOARCH=wasm go build -o web/fanamix.wasm ./webapp
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js

serve: web
	go run ./cmd/fanamix -window=none -addr $(ADDR)

site:
	node scripts/build-site.mjs

clean:
	rm -rf bin dist $(SITE) web/fanamix.wasm web/wasm_exec.js
