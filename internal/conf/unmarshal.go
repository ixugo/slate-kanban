package conf

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// SetupConfig 从文件读取内容初始化配置
func SetupConfig(v any, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(b, v)
}

// WriteConfig 将配置原子写回文件。
func WriteConfig(v any, path string) error {
	return writeConfig(v, path, 0o600)
}

func writeConfig(v any, path string, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := toml.NewEncoder(f).SetIndentTables(true).Encode(v); err != nil {
		return err
	}
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
