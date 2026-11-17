package data

import (
	"path/filepath"
	"testing"
	"time"

	"slate-kanban/internal/conf"
)

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
