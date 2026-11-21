# Slate 源码构建与开发启动

需要 Git、Go 1.27+、Node.js（建议使用 22 LTS，至少 22.22.2）和 Make。Docker 安装和看板使用见 [简体中文 README](../README.zh-CN.md)。

```sh
git clone https://github.com/ixugo/slate-kanban.git
cd slate-kanban
```

以下命令在仓库根目录执行。

## 前后端联调

分别打开两个终端，在第一个终端启动 Go 后端，在第二个终端启动前端开发服务器。

```sh
make run
```

```sh
make web/dev
```

打开 [http://localhost:5173](http://localhost:5173)。Vite 会把 `/api`、`/mcp`、`/health`、`/install.md` 和 `/skills` 转发到 `http://127.0.0.1:7070`。前端的 Make 目标会按需执行 `npm ci`。如果后端用了不同地址，启动前端时设置 `SLATE_BACKEND`，例如 `SLATE_BACKEND=http://127.0.0.1:7071 make web/dev`。

默认配置和数据位于 `~/.slate-kanban/`。为了使用独立的开发数据库，可将第一个终端的命令换成下面这条。Slate 首次运行会生成配置和 SQLite 数据库。

```sh
go run . -conf "$HOME/.slate-kanban-dev"
```

如果本机已有 Slate 占用 7070，在开发配置的 `[Server.HTTP]` 里改用其他端口，再重启后端，并把 `SLATE_BACKEND` 指向新端口。配置字段见 [带注释的配置样例](../configs/config.example.toml)。

数据库默认是 SQLite。把配置里的 `Data.Database.Dsn` 改成 `postgres://用户:密码@主机:5432/库名?sslmode=disable` 就切到 PostgreSQL，程序按前缀自动识别（`postgres://`、`postgresql://` 走 PostgreSQL，`sqlite://` 或只写文件路径走 SQLite），两种驱动都已内置，不需要构建标签。

## 构建并运行单文件程序

```sh
make build
./slate
```

打开 [http://127.0.0.1:7070](http://127.0.0.1:7070)。构建会把前端写到 `internal/web/spa/dist`，检查 gzip 体积预算，再用 `-tags embed_web,nomsgpack` 将前端嵌入 `./slate`（`nomsgpack` 去掉 gin 用不到的 msgpack，约省 5MB）。使用 `./slate -conf /path/to/dir` 可指定配置目录。

旧版本如果把数据放在启动目录下的 `configs/`，可继续用 `./slate -conf ./configs` 指向原目录。迁移目录前先停止 Slate，复制配置、数据库及已有的 `-wal`、`-shm` 文件和资源目录，再更新配置中的 `Data.Database.Dsn`、`Data.ResourceDir` 和 `Log.Dir`，使它们指向迁移后的位置。

`./slate -d` 会注册为系统服务并启动，`./slate -stop` 停止并删除该服务。Windows 是 Windows 服务（需管理员终端），Linux 是开机即启的 systemd 系统服务（会自动用 sudo 提权），macOS 是登录后启动的 LaunchAgent。

## 打发布包

发布包由 Makefile 打包，GitHub Actions 只负责准备环境、检出源码和上传，不自己读 git 哈希或拼编译参数。本地想试打包，可以执行：

```sh
make release/assets TAG=v1.2.3
```

产物放在 `dist/`：Windows amd64、Linux amd64、Linux arm64、macOS arm64 四个压缩包（`slate_<tag>_<系统>_<架构>.tar.gz` 或 `.zip`，解压后是同名目录，里面有程序和启动、停止脚本），加一个 `checksums.txt`。`TAG` 必须是 `v1.2.3` 这样的版本号，可以带 `-rc.1` 之类后缀。Linux 和 Windows 的程序会用 upx 压缩，本机没装 upx 时加 `UPX_BIN=` 跳过。

给旧版本补包时，工作流会同时检出两份代码：最新的 Makefile 放在 `tool/`，要打包的 tag 源码放在 `src/`，在 `src/` 里执行 `make -f ../tool/Makefile release/assets TAG=<tag>`，所以旧 tag 里没有这套 Makefile 也能打。

## 常用开发命令

| 命令 | 用途 |
| --- | --- |
| `make web/test` | 运行前端类型检查、oxlint 和单元测试 |
| `make web/build` | 构建前端并检查体积预算 |
| `go test ./internal/...` | 运行后端测试，默认使用 SQLite |
| `make test-postgres` | 用临时 Docker PostgreSQL 运行看板存储层和数据库连接层测试 |
| `go vet ./internal/... ./pkg/... .` | Go 静态检查 |
| `make docker/build` | 构建容器镜像 |
| `make release/assets TAG=v1.2.3` | 打四个平台的发布包和校验和到 `dist/` |

前端使用 React、Vite 和 Tailwind，见 [前端开发说明](../web/README.md)。后端沿用 [GoDDD](https://github.com/ixugo/goddd) 分层，进程入口在 `main.go`，看板领域和存储在 `internal/core/kanban/`，REST、MCP 和前端静态资源服务在 `internal/web/`。设计文档在 [`.cursor/scopes/`](../.cursor/scopes/)，REST 接口描述在 [`docs/api/`](api/)。
