package kanbandb

import (
	"database/sql"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
)

// legacyEmpty 是旧版两列里"没有内容"的几种写法。
var legacyEmpty = []string{"", "[]", "null"}

// legacyLabel 是旧版 boards.labels 里的一项。旧版的主键是 8 位十六进制串，新版不再用它，所以不读。
type legacyLabel struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// migrateLegacyTags 把旧版存在 boards.labels（标签库）和 cards.tags（卡片上的标签文字）里的数据，
// 搬进 tags 表和 cards.tag_ids：库里的标签按原顺序变成 tags 行，卡片上库外的文字补成没有颜色的标签，
// 再按文字把卡片的标签换成主键。每个看板一个事务，搬完就清空旧列，所以重复执行没有副作用；
// 旧列留在表里不删（AutoMigrate 本来就不删列），新代码不再读写它们。
func (d DB) migrateLegacyTags() error {
	m := d.db.Migrator()
	hasLabels, hasTags := m.HasColumn("boards", "labels"), m.HasColumn("cards", "tags")
	if !hasLabels && !hasTags {
		return nil
	}
	var boardIDs []int64
	if hasLabels {
		if err := d.db.Table("boards").Where("labels IS NOT NULL AND labels NOT IN ?", legacyEmpty).Pluck("id", &boardIDs).Error; err != nil {
			return err
		}
	}
	if hasTags {
		var ids []int64
		if err := d.db.Table("cards").Where("tags IS NOT NULL AND tags NOT IN ?", legacyEmpty).Distinct().Pluck("board_id", &ids).Error; err != nil {
			return err
		}
		boardIDs = append(boardIDs, ids...)
	}
	slices.Sort(boardIDs)
	for _, id := range slices.Compact(boardIDs) {
		err := d.db.Transaction(func(tx *gorm.DB) error {
			return migrateBoardTags(tx, id, hasLabels, hasTags)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// migrateBoardTags 搬一个看板的旧标签，调用方负责开事务。
func migrateBoardTags(tx *gorm.DB, boardID int64, hasLabels, hasTags bool) error {
	byText := make(map[string]int64)
	var next int64
	add := func(text, color string) (int64, error) {
		next++
		now := time.Now().UTC()
		t := &kanban.Tag{BoardID: boardID, Text: text, Color: color, Sort: next, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(t).Error; err != nil {
			return 0, err
		}
		byText[strings.ToLower(text)] = t.ID
		return t.ID, nil
	}
	if hasLabels {
		var raw sql.NullString
		if err := tx.Table("boards").Where("id = ?", boardID).Select("labels").Scan(&raw).Error; err != nil {
			return err
		}
		var labels []legacyLabel
		if raw.Valid && json.Unmarshal([]byte(raw.String), &labels) == nil {
			for _, l := range labels {
				if _, err := add(l.Name, l.Color); err != nil {
					return err
				}
			}
		}
		if err := tx.Table("boards").Where("id = ?", boardID).UpdateColumn("labels", nil).Error; err != nil {
			return err
		}
	}
	if !hasTags {
		return nil
	}
	return migrateCardTags(tx, boardID, byText, add)
}

// migrateCardTags 把看板每张卡片的标签文字换成主键，库里没有的文字通过 add 补成新标签。
func migrateCardTags(tx *gorm.DB, boardID int64, byText map[string]int64, add func(text, color string) (int64, error)) error {
	var rows []struct {
		ID   int64
		Tags sql.NullString
	}
	err := tx.Table("cards").Select("id", "tags").
		Where("board_id = ? AND tags IS NOT NULL AND tags NOT IN ?", boardID, legacyEmpty).Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		var names []string
		if json.Unmarshal([]byte(row.Tags.String), &names) != nil {
			continue
		}
		ids := make([]int64, 0, len(names))
		for _, name := range names {
			id, ok := byText[strings.ToLower(name)]
			if !ok {
				if id, err = add(name, ""); err != nil {
					return err
				}
			}
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		encoded, _ := json.Marshal(ids)
		err := tx.Table("cards").Where("id = ?", row.ID).
			UpdateColumns(map[string]any{"tag_ids": string(encoded), "tags": nil}).Error
		if err != nil {
			return err
		}
	}
	return nil
}
