package kanban

import (
	"bytes"
	"encoding/json"
	"strings"
)

// ParseProjectTemplateJSON 解析并校验外部上传的项目模板 JSON，通过才返回结构化结果。
func ParseProjectTemplateJSON(raw []byte) (*ProjectTemplateFile, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, invalidf("模板文件为空")
	}
	if len(raw) > MaxProjectTemplateImportBytes {
		return nil, invalidf("模板文件不能超过 %d 字节", MaxProjectTemplateImportBytes)
	}
	var file ProjectTemplateFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&file); err != nil {
		return nil, invalidf("模板 JSON 格式不合法")
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err == nil {
		return nil, invalidf("模板 JSON 只能有一个根对象")
	}
	if err := validateProjectTemplateFile(&file); err != nil {
		return nil, err
	}
	return &file, nil
}

// validateProjectTemplateFile 校验信封字段和各子项，不修改入参。
func validateProjectTemplateFile(file *ProjectTemplateFile) error {
	file.Name = strings.TrimSpace(file.Name)
	if file.Format != ProjectTemplateFormat {
		return invalidf("format 必须是 %s", ProjectTemplateFormat)
	}
	if file.Version != ProjectTemplateVersion {
		return invalidf("不支持的模板版本 %d", file.Version)
	}
	if err := checkLine("模板名称", file.Name, 1, maxProjectTemplateNameLen); err != nil {
		return err
	}
	if reservedProjectTemplateName(file.Name) {
		return invalidf("模板名称不能使用内置起步模板名 %s、%s、%s", TemplateDev, TemplateSimple, TemplateEmpty)
	}
	tags, err := normalizeTemplateTags(file.Tags)
	if err != nil {
		return err
	}
	file.Tags = tags
	engineers, err := normalizeTemplateEngineers(file.Engineers)
	if err != nil {
		return err
	}
	file.Engineers = engineers
	columns, err := normalizeTemplateColumns(file.Columns)
	if err != nil {
		return err
	}
	file.Columns = columns
	return nil
}

// reservedProjectTemplateName 判断名称是否是保留的内置起步模板名（区分大小写）。
func reservedProjectTemplateName(name string) bool {
	return name == TemplateDev || name == TemplateSimple || name == TemplateEmpty
}

// normalizeTemplateTags 整理并校验模板里的标签列表。
func normalizeTemplateTags(in []ProjectTemplateTag) ([]ProjectTemplateTag, error) {
	if len(in) > maxBoardTags {
		return nil, invalidf("模板最多 %d 个标签", maxBoardTags)
	}
	out := make([]ProjectTemplateTag, 0, len(in))
	seenText := make(map[string]bool)
	for _, t := range in {
		t.Text = strings.TrimSpace(t.Text)
		t.Color = strings.TrimSpace(t.Color)
		if t.Text != "" {
			if err := checkLine("标签", t.Text, 1, maxCardTagLen); err != nil {
				return nil, err
			}
			key := strings.ToLower(t.Text)
			if seenText[key] {
				return nil, invalidf("标签文字重复：%s", t.Text)
			}
			seenText[key] = true
		}
		if !validTagColor(t.Color) {
			return nil, invalidf("标签颜色不合法")
		}
		out = append(out, t)
	}
	return out, nil
}

// normalizeTemplateEngineers 整理并校验模板里的工程师；名称区分大小写判重。
func normalizeTemplateEngineers(in []ProjectTemplateEngineer) ([]ProjectTemplateEngineer, error) {
	if len(in) > maxBoardEngineers {
		return nil, invalidf("模板最多 %d 位工程师", maxBoardEngineers)
	}
	out := make([]ProjectTemplateEngineer, 0, len(in))
	names := make(map[string]bool, len(in))
	for _, e := range in {
		e.Name = strings.TrimSpace(e.Name)
		e.Prompt = strings.TrimSpace(e.Prompt)
		e.Avatar = strings.TrimSpace(e.Avatar)
		if err := checkLine("工程师名称", e.Name, 1, maxEngineerNameLen); err != nil {
			return nil, err
		}
		if err := checkText("角色提示词", e.Prompt, 1, maxEngineerPromptLen); err != nil {
			return nil, err
		}
		if len(e.Avatar) > maxEngineerAvatarLen || !avatarSeedPattern.MatchString(e.Avatar) {
			return nil, invalidf("头像种子只能是不超过 %d 位的字母、数字、下划线或连字符", maxEngineerAvatarLen)
		}
		if !validEngineerKind(e.Kind) {
			return nil, invalidf("工程师类型只能是 %s、%s、%s、%s 或 %s", EngineerDev, EngineerFrontend, EngineerBackend, EngineerUI, EngineerReview)
		}
		if names[e.Name] {
			return nil, invalidf("工程师名称重复：%s", e.Name)
		}
		names[e.Name] = true
		if e.Avatar == "" {
			e.Avatar = newAvatarSeed()
		}
		out = append(out, e)
	}
	return out, nil
}

// normalizeTemplateColumns 整理并校验模板里的列及 next_column 引用。
func normalizeTemplateColumns(in []ProjectTemplateColumn) ([]ProjectTemplateColumn, error) {
	if len(in) > maxColumnsPerBoard {
		return nil, ErrColumnLimit
	}
	out := make([]ProjectTemplateColumn, 0, len(in))
	names := make(map[string]bool, len(in))
	for _, c := range in {
		c.Name = strings.TrimSpace(c.Name)
		c.Instruction = strings.TrimSpace(c.Instruction)
		if err := checkLine("列名称", c.Name, 1, maxColumnNameLen); err != nil {
			return nil, err
		}
		if err := checkText("列指令", c.Instruction, 0, maxColumnInstructionLen); err != nil {
			return nil, err
		}
		key := strings.ToLower(c.Name)
		if names[key] {
			return nil, invalidf("列名称重复：%s", c.Name)
		}
		names[key] = true
		out = append(out, c)
	}
	n := len(out)
	for i, c := range out {
		if c.NextColumn < -1 || c.NextColumn >= n {
			return nil, invalidf("列 %q 的 next_column 不合法", c.Name)
		}
		if c.NextColumn == i {
			return nil, invalidf("列不能把自己的下一列指向自己")
		}
	}
	return out, nil
}
