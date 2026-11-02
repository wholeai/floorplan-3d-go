# 户型装修设计 —— 跨平台单文件构建
#
#   make            当前平台，产物 dist/floorplan3d
#   make release    六个平台全部打包（zip + SHA256SUMS），用于发 Release
#   make run        本地跑起来（http://localhost:8787）
#   make test       跑单元测试
#   make clean      清掉 dist/
#
# 交叉编译成立的前提：SQLite 驱动是纯 Go（glebarez/go-sqlite），不依赖 cgo，
# 所以 CGO_ENABLED=0 就能编出任何平台的静态单文件。

APP     := floorplan3d
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' | grep . || echo 1.0.0)
LDFLAGS := -s -w -X main.version=$(VERSION)
DIST    := dist

# GOOS/GOARCH：darwin 覆盖 Intel 与 Apple Silicon，windows 覆盖 x64 与 ARM
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build run test vet fmt clean release check e2e selftest

all: build

## 当前平台
build:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP) .
	@echo "→ $(DIST)/$(APP)  $(VERSION)"

## 所有平台 + 校验和，直接丢给 gh release
release: clean
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; ext=; \
	  [ "$$os" = "windows" ] && ext=".exe"; \
	  name=$(APP)_$(VERSION)_$${os}_$${arch}; \
	  echo "→ $$os/$$arch"; \
	  mkdir -p $(DIST)/$$name; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
	    -o $(DIST)/$$name/$(APP)$$ext . || exit 1; \
	  cp deploy/README-binary.txt $(DIST)/$$name/README.txt; \
	  cp LICENSE $(DIST)/$$name/; \
	  (cd $(DIST) && zip -qr $$name.zip $$name && rm -rf $$name) || exit 1; \
	done
	@cd $(DIST) && (shasum -a 256 *.zip 2>/dev/null || sha256sum *.zip) > SHA256SUMS
	@$(MAKE) --no-print-directory build
	@echo "── $(DIST)/ ──" && ls -1sh $(DIST)

run:
	go run .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

## 前端语法闸（不需要浏览器）
check:
	node scripts/check-frontend.mjs

## 前端自检：起服务 + 无头 Chrome 跑 ?selftest=1（需要本机装了 Chrome）
selftest: build
	@$(DIST)/$(APP) -addr 127.0.0.1:8790 -db /tmp/fp-selftest.db -no-open & echo $$! > /tmp/fp-selftest.pid
	@sleep 2
	@CHROME_BIN=$${CHROME_BIN:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}; \
	 "$$CHROME_BIN" --headless=new --disable-gpu --no-sandbox --virtual-time-budget=20000 \
	   --dump-dom "http://127.0.0.1:8790/?selftest=1" | grep -o 'SELFTEST [A-Z]*: [0-9]* passed, [0-9]* failed'
	@kill `cat /tmp/fp-selftest.pid` 2>/dev/null; rm -f /tmp/fp-selftest.pid

## 端到端：真点按钮（无头 Chrome + CDP，Node 22+）
e2e: build
	@rm -f /tmp/fp-e2e.db /tmp/fp-e2e.db-wal /tmp/fp-e2e.db-shm
	@$(DIST)/$(APP) -addr 127.0.0.1:8790 -db /tmp/fp-e2e.db -no-open & echo $$! > /tmp/fp-e2e.pid
	@sleep 2
	@node scripts/e2e.mjs --url http://127.0.0.1:8790/ || (kill `cat /tmp/fp-e2e.pid` 2>/dev/null; exit 1)
	@kill `cat /tmp/fp-e2e.pid` 2>/dev/null; rm -f /tmp/fp-e2e.pid

clean:
	rm -rf $(DIST)
