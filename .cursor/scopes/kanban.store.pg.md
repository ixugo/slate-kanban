# 看板存储层 PostgreSQL 验证（kanbandb）

## 需求目标

框架在 DSN 以 `postgres` 开头时使用 PostgreSQL，但 Slate 的看板存储实现和测试原先只在 SQLite 上跑过。
要在不复制用例的前提下，让同一套 `kanbandb` 存储测试在 SQLite 与 PostgreSQL 上都通过，并核对事务、行锁、JSON 字段和唯一约束在两种方言下的行为一致。

| 编号 | 问题场景 | 不解决的后果 | 解决方案 | 状态 |
|------|----------|-------------|---------|------|
| 1 | 默认 `go test` 依赖 docker | 开发机没装 Postgres 时日常测试失败 | 未设置 `SLATE_TEST_POSTGRES_DSN` 时只跑 SQLite；设置后 `ForEachBackend` 再多跑 postgres 子测试 | 实施 |
| 2 | 并行存储测试共用一个库 | 用例互相污染、顺序依赖 | 每个 postgres 子测试创建独立随机库，结束 `DROP DATABASE` | 实施 |
| 3 | 事务回滚后残留数据 | 回滚语义与线上一致性不可信 | 沿用 `TestRuleWithTx`、`TestBoardTxLockAndRollback` 等，在双后端执行 | 实施 |
| 4 | 并发抢卡片编号 | 两张卡同一 Key | `IncrCardSeq` 使用 `UPDATE … RETURNING`；`TestPostgresConcurrentIncrCardSeq` 压测 | 实施 |
| 5 | 并发领取同一张卡 | 两个 agent 同时持有 | PostgreSQL 行锁 + 看板事务；`TestPostgresConcurrentClaimOneWinner` | 实施 |
| 6 | JSON 标签/依赖中文与引号 | 读写字段损坏 | `TestPostgresJSONRoundTripSpecialChars` + `TestCardRoundTrip` 双后端 | 实施 |
| 7 | 唯一约束错误类型不一致 | 业务无法映射 409 | `orm.TranslateError` + `TestBoardSlugIsUnique` / `TestCardKeyIsUnique` 双后端 | 实施 |

## 方案

### 测试入口

- 环境变量 `SLATE_TEST_POSTGRES_DSN`：指向可连通的 Postgres（通常连 `postgres` 库，由测试代码 `CREATE DATABASE`）。
- `kanbantest.ForEachBackend`：每个现有存储测试包一层 `sqlite` / `postgres` 子测试。
- `make test-postgres`：启动 `postgres:16-alpine` 一次性容器、等待 `pg_isready`、导出 DSN、跑 `./internal/core/kanban/stores/kanbandb/...`，退出时 `docker rm -f`。

### 连接池

- SQLite 测试仍用单连接，暴露“事务未绑定连接”类错误。
- PostgreSQL 测试库连接池放大到 `MaxOpenConns=12`，供并发领取与自增用例使用。

### 方言要点

- 行锁：`board.Lock` 使用 `FOR UPDATE`，仅在 PostgreSQL 多连接下体现；SQLite 单连接写事务本身串行。
- 自增序号：`IncrCardSeq` 依赖 PostgreSQL 与 SQLite 均支持的 `RETURNING`。
- 筛选：`tag:` / `title:` 在领域层内存匹配（`EqualFold` / `containsFold`），不经过 SQL `LIKE`，无 PG/SQLite 大小写差异。

## 验证计划

| 测试名 | 说明 |
|--------|------|
| `Test*` in `kanbandb/*_test.go`（`ForEachBackend`） | 全部实体 CRUD、事务、唯一键、列表顺序 |
| `TestBeginRollback` | 事务开关与回滚 |
| `TestBoardTxLockAndRollback` | 锁行 + 回滚不留脏数据 |
| `TestRuleWithTx` | 规则事务提交/回滚 |
| `TestCursorAdvanceConcurrent` | 游标并发推进 |
| `TestPostgresJSONRoundTripSpecialChars` | JSON 中文与引号 |
| `TestPostgresConcurrentIncrCardSeq` | 仅 PG：并发编号不重复 |
| `TestPostgresConcurrentClaimOneWinner` | 仅 PG：并发领取唯一成功 |
| `TestPostgresUniqueErrorsMatchSQLite` | 重复 slug 的 `IsDuplicatedKey` |

本地验收：`go test -race -count=1 ./internal/...`（SQLite）；`make test-postgres` 连跑两遍；`docker ps -a` 无残留 `slate-pg-test-*` 容器。