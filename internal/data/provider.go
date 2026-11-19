package data

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/google/wire"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/system"
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

// getDialector 返回 dial 和 是否 sqlite；DSN 以 postgres 开头时走 PostgreSQL，
// 默认构建不含 PostgreSQL 驱动，此时返回错误，而不是把 DSN 当成文件路径建一个怪文件。
func getDialector(dsn string) (gorm.Dialector, bool, error) {
	if strings.HasPrefix(dsn, "postgres") {
		dial, err := postgresDialector(dsn)
		return dial, false, err
	}
	return sqlite.Open(sqliteDSN(dsn)), true, nil
}

// sqliteDSN 把配置里的文件路径转成带参数的 sqlite DSN。
// 相对路径以启动目录为基准，绝对路径原样使用；会先创建上级目录，
// 并开启 WAL 与 5 秒忙等待，降低读写互相阻塞、直接报 database is locked 的概率。
func sqliteDSN(path string) string {
	if !filepath.IsAbs(path) {
		path = filepath.Join(system.Getwd(), path)
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}
