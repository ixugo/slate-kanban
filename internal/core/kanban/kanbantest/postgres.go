package kanbantest

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ixugo/goddd/pkg/orm"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban/stores/kanbandb"
)

// 独立测试库名的前缀，只含小写字母，避免引号转义问题。
const pgTestDBPrefix = "slate_t_"

// postgresPingTimeout 是等待测试用 Postgres 接受连接的最长时间。
const postgresPingTimeout = 30 * time.Second

// postgresPingInterval 是两轮就绪探测之间的间隔。
const postgresPingInterval = 200 * time.Millisecond

// postgresMaxOpen 给并发用例留足连接，行锁和 goroutine 测试需要多路同时连库。
const postgresMaxOpen = 12

// postgresMaxIdle 保持在池里的空闲连接数，减少高频开关连接的开销。
const postgresMaxIdle = 6

// PostgresDSNEnv 是启用 PostgreSQL 存储测试的环境变量名；未设置时只跑 SQLite。
const PostgresDSNEnv = "SLATE_TEST_POSTGRES_DSN"

// postgresDSN 返回环境变量里配置的 PostgreSQL 地址，没配置时是空串。
func postgresDSN() string {
	return strings.TrimSpace(os.Getenv(PostgresDSNEnv))
}

// ForEachBackend 在 SQLite 上必跑；设置了 PostgresDSNEnv 时再加跑一轮 PostgreSQL。
// 子测试名固定为 sqlite/postgres，CI 和本地都容易对号入座。
func ForEachBackend(t *testing.T, fn func(t *testing.T, db kanbandb.DB)) {
	t.Helper()
	t.Run("sqlite", func(t *testing.T) {
		fn(t, NewDB(t))
	})
	if dsn := postgresDSN(); dsn != "" {
		t.Run("postgres", func(t *testing.T) {
			fn(t, newPostgresDB(t, dsn))
		})
	}
}

// ForEachRawBackend 与 ForEachBackend 类似，但回调拿到的是未迁移的裸连接和已迁移的 DB 封装。
// 给迁移、事务开关这类要先控迁移时机的用例用。
func ForEachRawBackend(t *testing.T, fn func(t *testing.T, raw *gorm.DB)) {
	t.Helper()
	t.Run("sqlite", func(t *testing.T) {
		fn(t, NewRawDB(t))
	})
	if dsn := postgresDSN(); dsn != "" {
		t.Run("postgres", func(t *testing.T) {
			fn(t, newPostgresRawDB(t, dsn))
		})
	}
}

// NewPostgresDB 用环境变量里的 DSN 创建独立测试库并完成迁移。
// 未配置 PostgresDSNEnv 时直接跳过当前测试，供必须多连接才能验证的专项用例（例如并发行锁）调用。
func NewPostgresDB(t *testing.T) kanbandb.DB {
	t.Helper()
	dsn := postgresDSN()
	if dsn == "" {
		t.Skip("未设置 " + PostgresDSNEnv + "，跳过 PostgreSQL 专项测试")
	}
	return newPostgresDB(t, dsn)
}

// newPostgresDB 在独立随机库上建好全部看板表，测试结束删库，互不影响。
func newPostgresDB(t *testing.T, baseDSN string) kanbandb.DB {
	t.Helper()
	raw := newPostgresRawDB(t, baseDSN)
	return kanbandb.NewDB(raw).AutoMigrate(true)
}

// newPostgresRawDB 创建随机库并返回 GORM 连接，不做表迁移。
func newPostgresRawDB(t *testing.T, baseDSN string) *gorm.DB {
	t.Helper()
	adminDSN, err := postgresAdminDSN(baseDSN)
	if err != nil {
		t.Fatalf("解析 Postgres DSN 失败: %v", err)
	}
	admin := openPostgresSQL(t, adminDSN)
	waitPostgresReady(t, admin)

	ctx := context.Background()
	dbName := pgTestDBPrefix + randomDBSuffix(10)
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+dbName); err != nil {
		t.Fatalf("创建测试库 %s 失败: %v", dbName, err)
	}

	testDSN, err := postgresDBDSN(baseDSN, dbName)
	if err != nil {
		t.Fatalf("拼装测试库 DSN 失败: %v", err)
	}
	db, err := orm.New(gormpostgres.New(gormpostgres.Config{DriverName: "pgx", DSN: testDSN}), orm.Config{
		MaxIdleConns: postgresMaxIdle,
		MaxOpenConns: postgresMaxOpen,
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				t.Logf("关闭测试库连接失败: %v", err)
			}
		}
		if _, err := admin.ExecContext(ctx, "DROP DATABASE "+dbName+" WITH (FORCE)"); err != nil {
			t.Logf("删除测试库 %s 失败: %v", dbName, err)
		}
		if err := admin.Close(); err != nil {
			t.Logf("关闭管理连接失败: %v", err)
		}
	})
	return db
}

// postgresAdminDSN 把业务 DSN 里的库名换成 postgres，用于建库和删库。
func postgresAdminDSN(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/postgres"
	return u.String(), nil
}

// postgresDBDSN 在原始 DSN 上替换库名为指定测试库。
func postgresDBDSN(dsn, dbName string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + dbName
	return u.String(), nil
}

// openPostgresSQL 用 database/sql 打开 Postgres，供管理连接使用。
func openPostgresSQL(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("打开 Postgres 管理连接失败: %v", err)
	}
	return db
}

// waitPostgresReady 轮询直到实例能接受连接或超时。
func waitPostgresReady(t *testing.T, db *sql.DB) {
	t.Helper()
	deadline := time.Now().Add(postgresPingTimeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := db.PingContext(ctx)
		cancel()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待 Postgres 就绪超时: %v", err)
		}
		time.Sleep(postgresPingInterval)
	}
}

// randomDBSuffix 生成只含小写字母的随机后缀，用作测试库名的一部分。
func randomDBSuffix(n int) string {
	b := make([]byte, n)
	for i := range b {
		// 测试库命名不承载安全语义，用非加密随机源即可。
		b[i] = byte('a' + rand.IntN(26))
	}
	return string(b)
}
