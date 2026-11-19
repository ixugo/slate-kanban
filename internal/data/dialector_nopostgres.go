//go:build !postgres

package data

import (
	"errors"

	"gorm.io/gorm"
)

// errPostgresUnsupported 说明默认构建没有 PostgreSQL 驱动，以及怎么得到带驱动的程序。
var errPostgresUnsupported = errors.New("当前程序不含 PostgreSQL 支持，请把 Data.Database.Dsn 改成 SQLite 文件路径，或用 -tags postgres 自行编译")

// postgresDialector 在默认构建里直接报错；pgx 驱动约占 1MB，发布包不带它。
func postgresDialector(string) (gorm.Dialector, error) {
	return nil, errPostgresUnsupported
}
