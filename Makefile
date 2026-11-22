# Makefile 使用文档
# https://www.gnu.org/software/make/manual/html_node/index.html

# 生成 RSA 私钥和公钥
# $ openssl genpkey -algorithm RSA -out private.key -pkeyopt rsa_keygen_bits:2048
# $ openssl rsa -in private.key -outform PEM -pubout -out public.key

# include .envrc
SHELL = /bin/bash

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/n] ' && read ans && [ $${ans:-N} = y ]

.PHONY: title
title:
	@echo -e "\033[34m$(content)\033[0m"


# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #


## init: 安装开发环境
init:
	@make title content="install dependencies..."
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/divan/expvarmon@latest
	go install github.com/ixugo/goddd/cmd/goddd@latest
	go install github.com/rakyll/hey@latest
	go install mvdan.cc/gofumpt@latest
	@make title content="Successed!"

## wire: 生成依赖注入代码
wire:
	go mod tidy
	go get github.com/google/wire/cmd/wire@latest
	go generate ./...
	go mod tidy

## run: 启动后端（开发模式，不带前端；前端用 make web/dev，或先 make web/build 再 go run -tags embed_web .）
.PHONY: run
run:
	go run .

# 前端依赖目录不存在，或比锁文件旧时才重新安装；下面的 web/ 目标都依赖它，所以不用手动先装依赖
web/node_modules: web/package-lock.json
	cd web && npm ci
	@touch web/node_modules

## web/install: 安装前端依赖（已是最新就跳过）
.PHONY: web/install
web/install: web/node_modules

## web/dev: 启动前端开发服务器（5173），接口和 MCP 转发到本机 7070 的后端
.PHONY: web/dev
web/dev: web/node_modules
	cd web && npm run dev

## web/test: 前端类型检查、代码检查和单元测试
.PHONY: web/test
web/test: web/node_modules
	cd web && npm run typecheck && npm run lint && npm run test

## web/build: 构建前端，产物写进 internal/web/spa/dist，并检查体积预算
.PHONY: web/build
web/build: web/node_modules
	cd web && npm run build

## expva/http: 监听网络请求指标
expva/http:
	expvarmon --ports="$${p:-8080}" -i 1s -vars="version,request,requests,responses,goroutines,panics,mem:memstats.HeapAlloc,mem:memstats.HeapSys,mem:memstats.Sys"

## expva/db: 监听数据库连接指标
expva/db:
	expvarmon --ports="$${p:-8080}" -i 5s -vars="databse.MaxOpenConnections,databse.OpenConnections,database.InUse,databse.Idle"

## statsviz: 打开 statsviz 可视化页面
# darwin 用 open 打开浏览器，windows 用 start 打开浏览器
.PHONY: statsviz
statsviz:
	@if [ "$(GOOS)" = "windows" ]; then \
		start http://$${p:-127.0.0.1:8080}/debug/statsviz/; \
	else \
		open http://$${p:-127.0.0.1:8080}/debug/statsviz/; \
	fi

# 发起 100 次请求，每次并发 50
# hey -n 100 -c 50 http://localhost:9999/healthcheck


# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## test-postgres: 启动一次性 PostgreSQL 容器并跑看板存储层和数据库连接层测试（需本机 docker）
.PHONY: test-postgres
test-postgres:
	@set -euo pipefail; \
	SUFFIX=$$(openssl rand -hex 4); \
	NAME="slate-pg-test-$$SUFFIX"; \
	PGPASS=$$(openssl rand -hex 12); \
	PORT=$$(python3 -c 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()'); \
	cleanup() { docker rm -f "$$NAME" >/dev/null 2>&1 || true; }; \
	trap cleanup EXIT; \
	echo "启动 Postgres 容器 $$NAME 映射端口 $$PORT ..."; \
	docker run -d --name "$$NAME" -e POSTGRES_PASSWORD="$$PGPASS" -p "127.0.0.1:$$PORT:5432" postgres:16-alpine >/dev/null; \
	deadline=$$(($$(date +%s) + 60)); \
	until docker exec "$$NAME" pg_isready -U postgres -q; do \
		if [ "$$(date +%s)" -ge "$$deadline" ]; then echo "Postgres 启动超时"; docker logs "$$NAME"; exit 1; fi; \
		sleep 0.3; \
	done; \
	export SLATE_TEST_POSTGRES_DSN="postgres://postgres:$$PGPASS@127.0.0.1:$$PORT/postgres?sslmode=disable"; \
	go test -race -count=1 ./internal/core/kanban/stores/kanbandb/... ./internal/data/...

## audit: 检查代码依赖/格式化/测试
.PHONY: audit
audit:
	@make title content='Formatting code...'
	gofumpt -l -w .
	@make title content='Vetting code...'
	go vet ./...
	@make title content='Running tests...'
	go test -race -vet=off ./...

## vendor: 整理并下载依赖
.PHONY: vendor
vendor:
	@make title content='Tidying and verifying module dependencies...'
	go mod tidy && go mod verify
	@make title content='Vendoring dependencies...'
	go mod vendor

# ==================================================================================== #
# VERSION
# ==================================================================================== #

# 版本号规则说明
# 1. 版本号使用 Git tag，格式为 v1.0.0。
# 2. 如果当前提交没有 tag，找到最近的 tag，计算从该 tag 到当前提交的提交次数。例如，最近的 tag 为 v1.0.1，当前提交距离它有 10 次提交，则版本号为 v1.0.11（v1.0.1 + 10 次提交）。
# 3. 如果没有任何 tag，则默认版本号为 v0.0.0，后续提交次数作为版本号的次版本号。

# Get the current module name
MODULE_NAME := $(shell pwd | awk -F "/" '{print $$NF}')
# Get the latest commit hash and date
HASH_AND_DATE := $(shell git log -n1 --pretty=format:"%h-%cd" --date=format:%y%m%d | awk '{print $1}')
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

# 如果想仅支持注释标签，可以去掉 --tags，否则会包含轻量标签
RECENT_TAG := $(shell git describe --tags --abbrev=0  2>&1 | grep -v -e "fatal" -e "Try" || echo "v0.0.0")

ifeq ($(RECENT_TAG),v0.0.0)
	COMMITS := $(shell git rev-list --count HEAD)
else
	COMMITS := $(shell git log --first-parent --format='%ae' $(RECENT_TAG)..$(BRANCH) | wc -l)
	COMMITS := $(shell echo $(COMMITS) | sed 's/ //g')
endif

# 从版本字符串中提取主版本号、次版本号和修订号
GIT_VERSION_MAJOR := $(shell echo $(RECENT_TAG) | cut -d. -f1 | sed 's/v//')
GIT_VERSION_MINOR := $(shell echo $(RECENT_TAG) | cut -d. -f2)
GIT_VERSION_PATCH := $(shell echo $(RECENT_TAG) | cut -d. -f3)

# windows 系统 git bash 没有 bc
# FINAL_PATCH := $(shell echo $(GIT_VERSION_PATCH) + $(COMMITS) | bc)
FINAL_PATCH := $(shell echo '$(GIT_VERSION_PATCH) $(COMMITS)' | awk '{print $$1 + $$2}')
VERSION := v$(GIT_VERSION_MAJOR).$(GIT_VERSION_MINOR).$(FINAL_PATCH)

# test:
# 	@echo ">>>${RECENT_TAG}"

## info: 查看构建版本相关信息
.PHONY: info
info:
	@echo "dir: $(MODULE_NAME)"
	@echo "version: $(VERSION)"
	@echo "branch $(BRANCH)"
	@echo "hash: $(HASH_AND_DATE)"


# ==================================================================================== #
# BUILD
# ==================================================================================== #

BUILD_DIR_ROOT	:= ./build
# 常量定义，如需修改，请修改命令内的变量
GOOS		= $(shell go env GOOS)
GOARCH		= $(shell go env GOARCH)
# 本项目的依赖都是纯 Go（SQLite 用 glebarez/sqlite），关闭 cgo 才能在本机交叉编译出静态程序
CGO_ENABLED	= 0
# 构建标签：embed_web 把前端嵌进程序；nomsgpack 去掉 gin 里用不到的 msgpack 编码，约省 5MB
GO_TAGS		= embed_web,nomsgpack
# 编译进程序的版本信息，build 和 build/local 共用
LDFLAGS		= -s -w \
	-X main.buildVersion=$(VERSION) \
	-X main.gitBranch=$(BRANCH) \
	-X main.gitHash=$(HASH_AND_DATE) \
	-X main.buildTime=$(shell date +%s) \
	-X main.release=true

IMAGE_NAME	:= $(MODULE_NAME):latest

## build: 构建前端并编译出带前端的单文件程序 ./slate（本机直接运行用）
.PHONY: build
build: web/build
	@echo 'Building $(VERSION) ./slate...'
	@CGO_ENABLED=$(CGO_ENABLED) go build -tags $(GO_TAGS) -trimpath -ldflags="$(LDFLAGS)" -o slate .
	@echo '>>> OK: ./slate'

## build/clean: 清理构建缓存目录
.PHONY: build/clean
build/clean:
	@rm -rf $(BUILD_DIR_ROOT)/*

## build/local: 构建指定系统和架构的应用（带前端），输出到 ./build/<系统>_<架构>/bin
.PHONY: build/local
build/local: web/build
	$(eval dir := $(BUILD_DIR_ROOT)/$(GOOS)_$(GOARCH))
	@echo 'Building $(VERSION) $(dir)...'
	@rm -rf $(dir)
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-tags $(GO_TAGS) \
		-trimpath \
		-ldflags="$(LDFLAGS)" -o=$(dir)/bin .
	@echo '>>> OK'

## build/linux: 构建 linux 应用
.PHONY: build/linux
BUILD_LINUX_AMD64_DIR := ./build/linux_amd64
build/linux:
	$(eval GOARCH := amd64)
	$(eval GOOS := linux)
	@make build/local GOOS=$(GOOS) GOARCH=$(GOARCH)

## build/windows: 构建 windows 应用
.PHONY: build/windows
BUILD_WINDOWS_AMD64_DIR := ./build/windows_amd64
build/windows:
	$(eval GOARCH := amd64)
	$(eval GOOS := windows)
	@make build/local GOOS=$(GOOS) GOARCH=$(GOARCH)

## docker/build: 用多阶段 Dockerfile 构建镜像（前端和后端都在镜像里编译，本机不需要装 node 和 go）
docker/build:
	@docker build --force-rm=true --build-arg VERSION=$(VERSION) -t $(IMAGE_NAME) .

docker/save:
	@docker save -o $(MODULE_NAME)_$(VERSION).tar $(IMAGE_NAME)

docker/push:
	@docker push $(IMAGE_NAME)

## docker/publish: 构建 amd64 和 arm64 两个架构的镜像并推送（会上传镜像，只在确认发布时运行）
docker/publish:
	@docker buildx build --force-rm=true --build-arg VERSION=$(VERSION) --platform linux/amd64,linux/arm64 -t $(IMAGE_NAME) --push .


# ==================================================================================== #
# RELEASE
# ==================================================================================== #

# 发布包的输出目录；打包用的临时目录也放在里面，打完就删
RELEASE_DIST := dist
# 每个平台只打一个包：程序和启动、停止脚本都放在压缩包根目录（旧版自动更新器要求程序在归档根目录）
# 没有启动脚本的历史 tag：v0.1.0 的源码和发布包里都没有 start/stop 脚本
RELEASE_NO_SCRIPT_TAGS := v0.1.0
# 写入 main.gitBranch 的值，标明这是 release 发布包（只用于版本信息展示，不影响更新检查）
RELEASE_GIT_BRANCH := release
# upx 压缩程序，只压 linux 和 windows；本机没装 upx 时传空（make release/assets TAG=... UPX_BIN=）可跳过压缩
UPX_BIN ?= upx
# 构建时间只取一次，四个平台的程序共用这一个值
RELEASE_BUILD_TIME := $(shell date +%s)
# 四个平台各一个目标，名字是 release/asset/<系统>_<架构>
RELEASE_ASSETS := release/asset/windows_amd64 release/asset/linux_arm64 release/asset/linux_amd64 release/asset/darwin_arm64

# TAG 由调用方给出（手动运行工作流时是用户填写的）。导出成环境变量后，recipe 里只用 shell 变量 $$TAG 读它，
# 不让 make 把它拼进命令字符串，带引号、分号的恶意值也就无法变成命令。
# make 还会把变量值里的 $(...) 当成表达式去执行，所以先看未展开的原始值（$(value) 不展开）：出现 $ 就直接报错
ifneq ($(findstring $$,$(value TAG)),)
$(error TAG 里不能出现 $$ 字符)
endif
export TAG

# 校验 TAG：不能为空，必须是 v1.2.3 这样的版本号（可带 -rc.1 之类后缀）。它会变成文件名，所以必须先挡住乱写的值
.PHONY: release/validate-tag
release/validate-tag:
	@if [ -z "$$TAG" ]; then echo '错误：需要 TAG，例如 make release/assets TAG=v1.2.3' >&2; exit 1; fi
	@printf '%s\n' "$$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-+.][0-9A-Za-z.-]+)*$$' || { echo "错误：TAG 格式不合法: $$TAG" >&2; exit 1; }

# 打包一个平台，参数依次是：系统、架构、程序文件名、归档后缀、启动脚本列表。
# 所有路径都相对当前目录（也就是被打包的源码目录），所以旧 tag 借用另一份 Makefile 时同样能用；
# 源码哈希在这里用 git 从当前目录读取，保证记录的是真正被打包的那份源码，而不是触发工作流的提交。
define release_package
	set -euo pipefail; \
	os='$(1)'; arch='$(2)'; bin='$(3)'; ext='$(4)'; \
	name="slate_$${TAG}_$${os}_$${arch}"; \
	package_dir="$(RELEASE_DIST)/$${name}"; \
	mkdir -p "$${package_dir}"; \
	source_commit=$$(git rev-parse --short=7 HEAD); \
	ldflags="-s -w -X main.buildVersion=$${TAG} -X main.gitBranch=$(RELEASE_GIT_BRANCH) -X main.gitHash=$${source_commit} -X main.buildTime=$(RELEASE_BUILD_TIME) -X main.release=true"; \
	CGO_ENABLED=$(CGO_ENABLED) GOOS="$${os}" GOARCH="$${arch}" go build -tags $(GO_TAGS) -trimpath -ldflags="$${ldflags}" -o "$${package_dir}/$${bin}" .; \
	if [ "$${os}" != "darwin" ] && [ -n '$(UPX_BIN)' ]; then \
		env -u UPX '$(UPX_BIN)' --best --lzma -q "$${package_dir}/$${bin}"; \
		env -u UPX '$(UPX_BIN)' -t -q "$${package_dir}/$${bin}"; \
	fi; \
	skip_scripts=0; \
	for t in $(RELEASE_NO_SCRIPT_TAGS); do if [ "$${t}" = "$${TAG}" ]; then skip_scripts=1; fi; done; \
	scripts=($(5)); \
	archive_entries=("$${bin}"); \
	if [ "$${skip_scripts}" -eq 0 ]; then \
		cp "$${scripts[@]}" "$${package_dir}/"; \
		archive_entries+=("$${scripts[@]}"); \
	fi; \
	source_dir="$${PWD}"; \
	archive() { \
		root="$$1"; out="$$2"; shift 2; \
		if [ "$${ext}" = ".zip" ]; then \
			( cd "$${root}" && zip -qr "$${source_dir}/$(RELEASE_DIST)/$${out}.zip" "$$@" ); \
		else \
			tar -C "$${root}" -czf "$(RELEASE_DIST)/$${out}.tar.gz" "$$@"; \
		fi; \
	}; \
	archive "$${package_dir}" "$${name}" "$${archive_entries[@]}"; \
	rm -rf "$${package_dir}"
endef

# windows/amd64：产出 .zip，程序是 slate.exe，脚本是 bat
.PHONY: release/asset/windows_amd64
release/asset/windows_amd64: release/validate-tag web/build
	@$(call release_package,windows,amd64,slate.exe,.zip,start.bat stop.bat)

# linux/arm64：产出 .tar.gz，脚本是 sh；upx 压缩后体积小很多
.PHONY: release/asset/linux_arm64
release/asset/linux_arm64: release/validate-tag web/build
	@$(call release_package,linux,arm64,slate,.tar.gz,start.sh stop.sh)

# linux/amd64：和 arm64 相同，覆盖最常见的服务器架构
.PHONY: release/asset/linux_amd64
release/asset/linux_amd64: release/validate-tag web/build
	@$(call release_package,linux,amd64,slate,.tar.gz,start.sh stop.sh)

# darwin/arm64：Apple 芯片的程序不用 upx 压缩（压缩会破坏代码签名，支持也不稳定）
.PHONY: release/asset/darwin_arm64
release/asset/darwin_arm64: release/validate-tag web/build
	@$(call release_package,darwin,arm64,slate,.tar.gz,start.sh stop.sh)

## release/assets: 构建四个平台的发布包和 checksums.txt 到 dist/（TAG 必填，例：make release/assets TAG=v1.2.3）
# 四个平台的包都打好之后，再对本次 TAG 的包生成校验和；优先用 sha256sum，没有（如 macOS）就用 shasum -a 256，两者输出格式相同
.PHONY: release/assets
release/assets: release/validate-tag $(RELEASE_ASSETS)
	@set -euo pipefail; \
	cd '$(RELEASE_DIST)'; \
	if command -v sha256sum >/dev/null 2>&1; then sum='sha256sum'; else sum='shasum -a 256'; fi; \
	$${sum} "slate_$${TAG}_"*.tar.gz "slate_$${TAG}_"*.zip > checksums.txt


# ==================================================================================== #
# PRODUCTION
# ==================================================================================== #

PRODUCTION_HOST = remoteHost

## release/push: 发布产品到服务器，仅上传文件
# 中小项目可以引入 CI/CD，也可以通过命令快速发布到测试服务器上。
release/push:
	@scp build/linux_amd64/bin $(PRODUCTION_HOST):/home/app/$(MODULE_NAME)
	@echo "push Successed"
