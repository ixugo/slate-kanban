package data

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"

	"slate-kanban/internal/conf"
)

// postgresDSNEnv 是测试用 PostgreSQL 地址的环境变量，和 kanbantest 用的是同一个；没设置时相关用例跳过。
const postgresDSNEnv = "SLATE_TEST_POSTGRES_DSN"

// TestSetupDBCleanupClosesDatabase 验证 SetupDB 返回的清理函数真的会关闭数据库连接，
// 否则服务退出时 SQLite 的 WAL 文件不会被合并回主文件。
func TestSetupDBCleanupClosesDatabase(t *testing.T) {
	bc := &conf.Bootstrap{Data: conf.Data{Database: conf.Database{
		Dsn:             filepath.Join(t.TempDir(), "slate.db"),
		ConnMaxLifetime: conf.Duration(time.Hour),
		SlowThreshold:   conf.Duration(200 * time.Millisecond),
	}}}

	db, cleanup, err := SetupDB(bc)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接失败: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("清理前应该能连上: %v", err)
	}

	cleanup()

	if err := sqlDB.Ping(); err == nil {
		t.Fatal("清理函数执行后连接应该已经关闭")
	}
}

// TestGetDialectorByPrefix 验证按 DSN 前缀自动识别数据库：postgres:// 和 postgresql:// 走 PostgreSQL（不分大小写），
// sqlite:// 或没有前缀走 SQLite；只是文件名以 postgres 开头的路径仍然是 SQLite，不能被误判。
func TestGetDialectorByPrefix(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name       string
		dsn        string
		wantSQLite bool
	}{
		{"postgres 连接串", "postgres://u:p@127.0.0.1:5432/db?sslmode=disable", false},
		{"postgresql 连接串", "postgresql://u@h/db", false},
		{"大写 scheme", "POSTGRES://u@h/db", false},
		{"前后有空白", "  postgres://u@h/db\n", false},
		{"sqlite 前缀加绝对路径", "sqlite://" + filepath.Join(dir, "a.db"), true},
		{"sqlite 大写前缀", "SQLite://" + filepath.Join(dir, "b.db"), true},
		{"只有路径", filepath.Join(dir, "c.db"), true},
		{"文件名以 postgres 开头的路径", filepath.Join(dir, "postgres-data.db"), true},
		{"相对路径", "postgres-data.db", true},
	}
	for _, tc := range cases {
		dial, isSQLite, err := getDialector(tc.dsn)
		if err != nil || dial == nil {
			t.Fatalf("%s：不应该报错: %v", tc.name, err)
		}
		if isSQLite != tc.wantSQLite {
			t.Errorf("%s：isSQLite = %v，期望 %v", tc.name, isSQLite, tc.wantSQLite)
		}
		wantName := "postgres"
		if tc.wantSQLite {
			wantName = "sqlite"
		}
		if dial.Name() != wantName {
			t.Errorf("%s：方言是 %q，期望 %q", tc.name, dial.Name(), wantName)
		}
	}
}

// TestGetDialectorSQLitePath 验证三种写法最终打开的是同一个文件路径：sqlite:// 前缀（不分大小写）会被去掉，
// 不会把前缀当成目录名的一部分。
func TestGetDialectorSQLitePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	for _, dsn := range []string{path, "sqlite://" + path, "SQLite://" + path} {
		dial, _, err := getDialector(dsn)
		if err != nil {
			t.Fatalf("%q 不应该报错: %v", dsn, err)
		}
		lite, ok := dial.(*sqlite.Dialector)
		if !ok || !strings.HasPrefix(lite.DSN, path+"?") {
			t.Errorf("%q 打开的路径不对: %T %+v", dsn, dial, dial)
		}
	}
}

// TestGetDialectorPostgresKeepsDSN 验证 PostgreSQL 的连接串原样交给 pgx 驱动，没有被改写。
func TestGetDialectorPostgresKeepsDSN(t *testing.T) {
	const dsn = "postgresql://u:p%40ss@db.example.com:5432/slate?sslmode=require&application_name=slate"
	dial, _, err := getDialector(dsn)
	if err != nil {
		t.Fatalf("不应该报错: %v", err)
	}
	pg, ok := dial.(*postgres.Dialector)
	if !ok {
		t.Fatalf("应该是 PostgreSQL 方言，实际 %T", dial)
	}
	if pg.DriverName != "pgx" || pg.DSN != dsn {
		t.Errorf("驱动或 DSN 不对: %q %q", pg.DriverName, pg.DSN)
	}
}

// TestGetDialectorRejectsEmpty 验证空 DSN 和只有 sqlite:// 前缀的 DSN 会被拒绝，
// 否则空路径会让 SQLite 去打开启动目录本身。
func TestGetDialectorRejectsEmpty(t *testing.T) {
	for _, dsn := range []string{"", "   ", "sqlite://", "SQLITE://  "} {
		if _, _, err := getDialector(dsn); err == nil {
			t.Errorf("%q 应该报错", dsn)
		}
	}
}

// TestPostgresDriverLinkedInDefaultBuild 验证 pgx 驱动在默认构建里就已经注册，
// 不需要 -tags postgres：这是「默认同时支持 SQLite 和 PostgreSQL」的底线。
func TestPostgresDriverLinkedInDefaultBuild(t *testing.T) {
	if !slices.Contains(sql.Drivers(), "pgx") {
		t.Fatalf("默认构建里没有注册 pgx 驱动: %v", sql.Drivers())
	}
}

// TestSetupDBPostgresUnreachable 验证 postgres DSN 真的会去连 PostgreSQL：连不上就报错，
// 不会被当成文件路径悄悄建出一个 SQLite 文件。
func TestSetupDBPostgresUnreachable(t *testing.T) {
	bc := &conf.Bootstrap{Data: conf.Data{Database: conf.Database{
		Dsn:             "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1",
		MaxIdleConns:    1,
		MaxOpenConns:    1,
		ConnMaxLifetime: conf.Duration(time.Hour),
		SlowThreshold:   conf.Duration(200 * time.Millisecond),
	}}}
	if _, cleanup, err := SetupDB(bc); err == nil {
		cleanup()
		t.Fatal("连不上 PostgreSQL 时应该报错")
	}
}

// TestSetupDBPostgres 验证默认构建的 SetupDB 能连上真实的 PostgreSQL 并正常关闭。
// 需要环境变量 SLATE_TEST_POSTGRES_DSN（make test-postgres 会设置），没设置时跳过。
func TestSetupDBPostgres(t *testing.T) {
	dsn := os.Getenv(postgresDSNEnv)
	if dsn == "" {
		t.Skipf("未设置 %s，跳过 PostgreSQL 连通性测试", postgresDSNEnv)
	}
	bc := &conf.Bootstrap{Data: conf.Data{Database: conf.Database{
		Dsn:             dsn,
		MaxIdleConns:    2,
		MaxOpenConns:    4,
		ConnMaxLifetime: conf.Duration(time.Hour),
		SlowThreshold:   conf.Duration(200 * time.Millisecond),
	}}}
	db, cleanup, err := SetupDB(bc)
	if err != nil {
		t.Fatalf("连接 PostgreSQL 失败: %v", err)
	}
	if db.Dialector.Name() != "postgres" {
		t.Errorf("方言应该是 postgres，实际 %q", db.Dialector.Name())
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接失败: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("清理前应该能连上: %v", err)
	}
	cleanup()
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("清理函数执行后连接应该已经关闭")
	}
}
