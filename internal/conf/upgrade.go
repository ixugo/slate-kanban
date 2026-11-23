package conf

import (
	"fmt"
	"os"
	"strings"
)

// CurrentConfigVersion is written to new configurations and to migrated legacy configurations.
const CurrentConfigVersion = 1

// UpgradeConfig updates the on-disk DSN once, before opening the database.
// The startup path holds the configuration directory lock during this call.
func UpgradeConfig(bc *Bootstrap, path string) error {
	if bc.ConfigVersion >= CurrentConfigVersion {
		return nil
	}

	var document map[string]any
	if err := SetupConfig(&document, path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	dsn := bc.Data.Database.Dsn
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(dsn)), "postgres://") {
		dsn, err = DefaultDatabasePath()
		if err != nil {
			return err
		}
		data, err := configTable(document, "Data")
		if err != nil {
			return err
		}
		db, err := configTable(data, "Database")
		if err != nil {
			return err
		}
		key, err := configKey(db, "Dsn")
		if err != nil {
			return err
		}
		db[key] = dsn
	}

	key, err := configKey(document, "ConfigVersion")
	if err != nil {
		return err
	}
	document[key] = CurrentConfigVersion
	if err := writeConfig(document, path, info.Mode().Perm()); err != nil {
		return err
	}
	bc.ConfigVersion = CurrentConfigVersion
	bc.Data.Database.Dsn = dsn
	return nil
}

// configTable reuses the spelling in TOML. Struct decoding also accepts differently cased keys.
func configTable(parent map[string]any, name string) (map[string]any, error) {
	key, err := configKey(parent, name)
	if err != nil {
		return nil, err
	}
	value, exists := parent[key]
	if !exists {
		table := make(map[string]any)
		parent[key] = table
		return table, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("配置字段 %s 应为表", name)
	}
	return table, nil
}

func configKey(table map[string]any, name string) (string, error) {
	var key string
	for candidate := range table {
		if strings.EqualFold(candidate, name) {
			if key != "" {
				return "", fmt.Errorf("配置字段 %s 有多个大小写变体", name)
			}
			key = candidate
		}
	}
	if key == "" {
		return name, nil
	}
	return key, nil
}
