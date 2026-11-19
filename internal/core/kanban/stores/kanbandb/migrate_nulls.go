package kanbandb

// jsonArrayColumn 是存 JSON 数组的一列：这些列过去允许 NULL，现在要求非空、默认空数组。
type jsonArrayColumn struct{ table, column string }

// jsonArrayColumns 列出所有存 JSON 数组的列，和模型里 serializer:json 的字段一一对应。
var jsonArrayColumns = []jsonArrayColumn{
	{"cards", "waits_on"},
	{"cards", "tag_ids"},
	{"rules", "actions"},
}

// fillNullJSONColumns 在表结构同步之前，把 JSON 数组列里的 NULL 和字面量 null 改成空数组 []。
// 模型要求这些列 NOT NULL，旧库里留着 NULL 的话，改列定义时整张表会重建失败。
// 表或列还不存在（全新的库）就跳过；重复执行没有副作用。
func (d DB) fillNullJSONColumns() error {
	m := d.db.Migrator()
	for _, c := range jsonArrayColumns {
		if !m.HasTable(c.table) || !m.HasColumn(c.table, c.column) {
			continue
		}
		err := d.db.Exec("UPDATE " + c.table + " SET " + c.column + " = '[]' WHERE " + c.column + " IS NULL OR " + c.column + " = 'null' OR " + c.column + " = ''").Error
		if err != nil {
			return err
		}
	}
	return nil
}

// dropLegacyTagColumns 在旧标签数据搬完以后，删掉旧的 boards.labels 和 cards.tags 两列：
// 它们允许 NULL 又不再有人读写，留着只会让库里残留可空列。列不存在就什么都不做。
// 直接用 ALTER TABLE ... DROP COLUMN（SQLite 3.35 起和 PostgreSQL 都支持）；
// 不用 gorm 的 DropColumn，因为 SQLite 驱动靠正则改建表语句，删最后一列时会失败。
func (d DB) dropLegacyTagColumns() error {
	m := d.db.Migrator()
	for _, c := range []jsonArrayColumn{{"boards", "labels"}, {"cards", "tags"}} {
		if m.HasColumn(c.table, c.column) {
			if err := d.db.Exec("ALTER TABLE " + c.table + " DROP COLUMN " + c.column).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
