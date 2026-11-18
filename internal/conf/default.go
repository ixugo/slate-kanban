package conf

import (
	"time"

	"github.com/ixugo/goddd/pkg/orm"
)

// DefaultConfig 返回首次生成配置文件时用的默认值：只监听本机，Host 白名单为空。
func DefaultConfig() Bootstrap {
	return Bootstrap{
		Server: Server{
			HTTP: ServerHTTP{
				Host:         "127.0.0.1",
				Port:         7070,
				Timeout:      Duration(30 * time.Second),
				JwtSecret:    orm.GenerateRandomString(32),
				AllowedHosts: nil,
				PProf: ServerPPROF{
					Enabled:   true,
					AccessIps: []string{"::1", "127.0.0.1"},
				},
			},
		},
		Data: Data{
			Database: Database{
				Dsn:             "./configs/data.db",
				MaxIdleConns:    10,
				MaxOpenConns:    50,
				ConnMaxLifetime: Duration(6 * time.Hour),
				SlowThreshold:   Duration(200 * time.Millisecond),
			},
		},
		Log: Log{
			Dir:          "./logs",
			Level:        "info",
			MaxAge:       7,
			RotationTime: Duration(8 * time.Hour),
			MaxSize:      50,
			Compress:     false,
			MaxBackups:   0,
		},
	}
}
