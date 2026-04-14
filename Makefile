TAG_NAME?=$(shell git describe --tags)
APP_NAME=BiliShareMall
WEB_APP_NAME=BiliShareMallWeb
GOCACHE_DIR?=$(CURDIR)/.cache/go-build
GOMODCACHE_DIR?=$(CURDIR)/.cache/gomod
WEB_ADDR?=:3761

.PHONY: run,run-web,dev-web,embed
run:
	@echo "Running..."
	wails dev -loglevel Info -tags fts5 -race

run-web:
	@mkdir -p "$(GOCACHE_DIR)" "$(GOMODCACHE_DIR)"
	pnpm --dir frontend build
	GOCACHE="$(GOCACHE_DIR)" GOMODCACHE="$(GOMODCACHE_DIR)" BSM_HTTP_ADDR="$(WEB_ADDR)" go run -tags fts5 ./cmd/web

dev-web:
	@mkdir -p "$(GOCACHE_DIR)" "$(GOMODCACHE_DIR)"
	/bin/zsh -lc 'trap "kill 0" EXIT; GOCACHE="$(GOCACHE_DIR)" GOMODCACHE="$(GOMODCACHE_DIR)" BSM_HTTP_ADDR="$(WEB_ADDR)" go run -tags fts5 ./cmd/web & pnpm --dir frontend dev --host 0.0.0.0'

.PHONY: install_deps
embed:
	go:embed all:frontend/dist

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build
build:
	 wails build  -nsis -tags fts5

.PHONY: build-web
build-web:
	@mkdir -p build/bin "$(GOCACHE_DIR)" "$(GOMODCACHE_DIR)"
	pnpm --dir frontend build
	GOCACHE="$(GOCACHE_DIR)" GOMODCACHE="$(GOMODCACHE_DIR)" go build -tags fts5 -o build/bin/$(WEB_APP_NAME) ./cmd/web

.PHONY: build-verify
build-verify:
	bash ./bin/build_verify.sh

.PHONY: debug
debug:
	wails build  -nsis -tags fts5 -windowsconsole -debug

.PHONY: autotag
autotag:
	@bash -c "bin/autotag"

.PHONY: dict
dict:
	go-bindata -o internal/domain/dict.go ./dict
