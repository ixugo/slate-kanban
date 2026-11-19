package conf

import (
	"path/filepath"
	"time"
)

// DefaultConfig 返回首次生成配置文件时用的默认值：监听所有 IPv4 网卡的 7070 端口，Host 白名单为空。
// dir 是配置目录，SQLite 数据库（data.db）和日志（logs/）默认都放在它下面，写成绝对路径，和启动目录无关。
func DefaultConfig(dir string) Bootstrap {
	return Bootstrap{
		Server: Server{
			HTTP: ServerHTTP{
				Host:         "0.0.0.0",
				Port:         7070,
				Timeout:      Duration(30 * time.Second),
				AllowedHosts: nil,
				PProf: ServerPPROF{
					Enabled:   true,
					AccessIps: []string{"::1", "127.0.0.1"},
				},
			},
		},
		Data: Data{
			ResourceDir: filepath.Join(dir, "resource"),
			Database: Database{
				Dsn:             filepath.Join(dir, "data.db"),
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
