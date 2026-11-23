package data

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/google/wire"
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"slate-kanban/internal/conf"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(SetupDB)

// SetupDB 初始化数据存储。返回的清理函数在服务退出时调用，会真正关闭数据库连接，
// 这样 SQLite 能把 WAL 里还没合并的内容写回主文件。
func SetupDB(c *conf.Bootstrap) (*gorm.DB, func(), error) {
	cfg := c.Data.Database
	dial, isSQLite, err := getDialector(cfg.Dsn)
	if err != nil {
		return nil, func() {}, err
	}
	if isSQLite {
		cfg.MaxIdleConns = 1
		cfg.MaxOpenConns = 1
	}
	db, err := orm.New(dial, orm.Config{
		MaxIdleConns:    int(cfg.MaxIdleConns),
		MaxOpenConns:    int(cfg.MaxOpenConns),
		ConnMaxLifetime: cfg.ConnMaxLifetime.Duration(),
		SlowThreshold:   cfg.SlowThreshold.Duration(),
	})
	if err != nil {
		return nil, func() {}, err
	}
	return db, func() { closeDB(db) }, nil
}

// closeDB 关闭 gorm 底层的连接池；失败只记日志，因为这时服务已经在退出，没有别的补救办法。
func closeDB(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("取数据库连接失败，无法关闭", "err", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		slog.Error("关闭数据库失败", "err", err)
	}
}

// 识别 DSN 种类用的前缀，不区分大小写。
// PostgreSQL 用 URL 形式的连接串；SQLite 可以写 sqlite:// 加文件路径，也可以什么前缀都不写，直接写文件路径。
const (
	postgresPrefix   = "postgres://"
	postgresqlPrefix = "postgresql://"
	sqlitePrefix     = "sqlite://"
)

var errEmptySQLitePath = errors.New("sqlite:// 后必须填写数据库文件路径（Data.Database.Dsn）")

// getDialector 按 DSN 的前缀选数据库，返回 dial 和是否 sqlite：
//   - 空值：使用 ~/.slate-kanban/data.db；
//   - postgres:// 或 postgresql:// 开头：PostgreSQL，连接串原样交给 pgx 驱动；
//   - sqlite:// 开头：去掉前缀，后面是 SQLite 文件路径；
//   - 其他：整串当成 SQLite 文件路径。
//
// 两种驱动都编译进程序，不需要构建标签，换数据库只改配置里的 DSN。
func getDialector(dsn string) (gorm.Dialector, bool, error) {
	dsn = strings.TrimSpace(dsn)
	switch {
	case dsn == "":
		path, err := conf.DefaultDatabasePath()
		if err != nil {
			return nil, false, fmt.Errorf("定位默认 SQLite 数据库: %w", err)
		}
		dsn = path
	case hasPrefixFold(dsn, postgresPrefix) || hasPrefixFold(dsn, postgresqlPrefix):
		return postgres.New(postgres.Config{DriverName: "pgx", DSN: dsn}), false, nil
	case hasPrefixFold(dsn, sqlitePrefix):
		dsn = strings.TrimSpace(dsn[len(sqlitePrefix):])
		if dsn == "" {
			return nil, false, errEmptySQLitePath
		}
	}
	dsn, err := sqliteDSN(dsn)
	if err != nil {
		return nil, false, err
	}
	return sqlite.Open(dsn), true, nil
}

// hasPrefixFold 判断 s 是否以 prefix 开头，忽略大小写（scheme 本来就不区分大小写）。
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// sqliteDSN 把配置里的文件路径转成带参数的 sqlite DSN。
// 相对路径以可执行文件目录为基准。先创建上级目录，再开启 WAL 与 5 秒忙等待。
func sqliteDSN(path string) (string, error) {
	if !filepath.IsAbs(path) {
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("定位 SQLite 相对路径的程序目录: %w", err)
		}
		exe, err = filepath.EvalSymlinks(exe)
		if err != nil {
			return "", fmt.Errorf("解析 SQLite 相对路径的程序目录: %w", err)
		}
		path = filepath.Join(filepath.Dir(exe), path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("创建 SQLite 数据库目录: %w", err)
	}
	return path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", nil
}
