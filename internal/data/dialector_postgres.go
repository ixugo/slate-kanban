//go:build postgres

package data

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// postgresDialector 返回 PostgreSQL 方言。pgx 驱动体积大，只在 -tags postgres 的构建里才链接进程序。
func postgresDialector(dsn string) (gorm.Dialector, error) {
	return postgres.New(postgres.Config{
		DriverName: "pgx",
		DSN:        dsn,
	}), nil
}
