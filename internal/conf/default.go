package conf

import (
	"path/filepath"
	"time"
)

// DefaultConfig 返回首次生成配置文件时用的默认值：监听所有 IPv4 网卡的 7070 端口。
// dir 用于默认资源和日志目录。数据库 DSN 留空，连接时选择默认 SQLite 文件。
func DefaultConfig(dir string) Bootstrap {
	return Bootstrap{
		ConfigVersion: CurrentConfigVersion,
		Server: Server{
			HTTP: ServerHTTP{
				Host:    "0.0.0.0",
				Port:    7070,
				Timeout: Duration(30 * time.Second),
				PProf: ServerPPROF{
					Enabled:   true,
					AccessIps: []string{"::1", "127.0.0.1"},
				},
			},
		},
		Data: Data{
			ResourceDir: filepath.Join(dir, "resource"),
			Database: Database{
				MaxIdleConns:    10,
				MaxOpenConns:    50,
				ConnMaxLifetime: Duration(6 * time.Hour),
				SlowThreshold:   Duration(200 * time.Millisecond),
			},
		},
		Log: Log{
			Dir:          filepath.Join(dir, "logs"),
			Level:        "info",
			MaxAge:       7,
			RotationTime: Duration(8 * time.Hour),
			MaxSize:      50,
			Compress:     false,
			MaxBackups:   0,
		},
	}
}
