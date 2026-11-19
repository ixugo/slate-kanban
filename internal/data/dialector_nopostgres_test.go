//go:build !postgres

package data

import "testing"

// TestPostgresDSNRejectedWithoutDriver 默认构建遇到 postgres DSN 必须报错，不能当成文件路径去建库文件。
func TestPostgresDSNRejectedWithoutDriver(t *testing.T) {
	for _, dsn := range []string{"postgres://u:p@127.0.0.1:5432/db", "postgresql://u@h/db"} {
		if _, _, err := getDialector(dsn); err == nil {
			t.Fatalf("%q 应该报错", dsn)
		}
	}
}

// TestSQLitePathStillWorks 普通文件路径仍然走 SQLite。
func TestSQLitePathStillWorks(t *testing.T) {
	dial, isSQLite, err := getDialector(t.TempDir() + "/x.db")
	if err != nil || !isSQLite || dial == nil {
		t.Fatalf("应返回 sqlite 方言: %v %v %v", dial, isSQLite, err)
	}
}
