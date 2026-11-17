package data

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"github.com/google/wire"
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/system"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"slate-kanban/internal/conf"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(SetupDB)

// SetupDB 初始化数据存储
func SetupDB(c *conf.Bootstrap) (*gorm.DB, func(), error) {
	cfg := c.Data.Database
	dial, isSQLite := getDialector(cfg.Dsn)
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

	closeFn := func() {
		sqlDB, err := db.DB()
		if err != nil {
			_ = sqlDB.Close()
		}
	}

	return db, closeFn, err
}

// getDialector 返回 dial 和 是否 sqlite
func getDialector(dsn string) (gorm.Dialector, bool) {
	if strings.HasPrefix(dsn, "postgres") {
		return postgres.New(postgres.Config{
			DriverName: "pgx",
			DSN:        dsn,
		}), false
	}
	return sqlite.Open(sqliteDSN(dsn)), true
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
