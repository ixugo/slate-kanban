package kanban

import "encoding/json"

// MarshalJSONOmitField 把 v 编成 JSON 后去掉顶层的一个字段，用于导出等要屏蔽个别字段的场景。
func MarshalJSONOmitField(v any, omit string) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return b, nil
	}
	delete(m, omit)
	return json.Marshal(m)
}
