package kanban

import "strings"

const (
	// maxColumnNameLen 列名称的最大字符数，和数据库列长度一致。
	maxColumnNameLen = 60
	// maxColumnInstructionLen 给 agent 的列指令的最大字符数。指令会进入 agent 的上下文，所以不能无限长。
	maxColumnInstructionLen = 4000
	// maxColumnsPerBoard 单个看板最多有几列。列太多会塞满页面和 agent 的上下文。
	maxColumnsPerBoard = 30
)

// CreateColumnInput 是新增列的入参。新列总是加在最右边。
type CreateColumnInput struct {
	Name         string `json:"name"`
	Instruction  string `json:"instruction"`
	IsDone       bool   `json:"is_done"`
	NextColumnID int64  `json:"next_column_id"` // 0 表示没有下一列
}

// UpdateColumnInput 是修改列的入参。指针字段为 nil 表示不修改；NextColumnID 填 0 表示清除。
type UpdateColumnInput struct {
	Name         *string `json:"name"`
	Instruction  *string `json:"instruction"`
	IsDone       *bool   `json:"is_done"`
	NextColumnID *int64  `json:"next_column_id"`
}

// normalize 整理并校验新增入参：名称去掉首尾空白，再检查各字段长度。
func (in *CreateColumnInput) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	if err := checkLine("列名称", in.Name, 1, maxColumnNameLen); err != nil {
		return err
	}
	return checkText("agent 指令", in.Instruction, 0, maxColumnInstructionLen)
}

// normalize 整理并校验修改入参，只检查出现的字段。
func (in *UpdateColumnInput) normalize() error {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		in.Name = &trimmed
		if err := checkLine("列名称", trimmed, 1, maxColumnNameLen); err != nil {
			return err
		}
	}

	if in.Instruction != nil {
		return checkText("agent 指令", *in.Instruction, 0, maxColumnInstructionLen)
	}
	return nil
}

// apply 把入参里出现的字段写到列上，返回真正发生变化的字段名（和 JSON 字段名一致）。
// 它不检查名称是否重复、下一列是否合法，这些要用到看板里的其他列，由调用方在 apply 之前检查。
func (in UpdateColumnInput) apply(c *Column) []string {
	var changed []string
	if in.Name != nil && *in.Name != c.Name {
		c.Name = *in.Name
		changed = append(changed, "name")
	}
	if in.Instruction != nil && *in.Instruction != c.Instruction {
		c.Instruction = *in.Instruction
		changed = append(changed, "instruction")
	}
	if in.IsDone != nil && *in.IsDone != c.IsDone {
		c.IsDone = *in.IsDone
		changed = append(changed, "is_done")
	}
	if in.NextColumnID != nil && *in.NextColumnID != c.NextColumnID {
		c.NextColumnID = *in.NextColumnID
		changed = append(changed, "next_column_id")
	}
	return changed
}

// columnNameTaken 判断名称是否已被看板里除 exceptID 之外的列使用，不区分大小写。
// 新增时 exceptID 传 0；修改时传自己，这样只改大小写不会撞上自己。
func columnNameTaken(cols []*Column, name string, exceptID int64) bool {
	for _, col := range cols {
		if col.ID != exceptID && strings.EqualFold(col.Name, name) {
			return true
		}
	}
	return false
}

// checkNextColumn 校验"下一列"的设置：0 表示没有；否则必须是同一看板里的另一列。
// selfID 是正在修改的列，新增时传 0。
func checkNextColumn(cols []*Column, selfID, nextID int64) error {
	if nextID == 0 {
		return nil
	}
	if nextID == selfID {
		return invalidf("下一列不能是它自己")
	}
	for _, col := range cols {
		if col.ID == nextID {
			return nil
		}
	}
	return invalidf("下一列必须是同一个看板里的另一列")
}
