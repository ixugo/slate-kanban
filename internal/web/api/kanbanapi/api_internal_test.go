package kanbanapi

import (
	"reflect"
	"testing"
)

// TestPathFieldsIgnoreBodyAndQuery 验证所有路径参数字段都标了 json:"-" 和 form:"-"：
// 缺一个，请求体或查询参数里的同名字段就能覆盖 URL 里的编号。新增路径参数结构体时要加进这个列表。
func TestPathFieldsIgnoreBodyAndQuery(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeFor[boardPath](), reflect.TypeFor[columnPath](), reflect.TypeFor[cardPath](),
		reflect.TypeFor[commentPath](), reflect.TypeFor[rulePath](),
	}
	for _, typ := range types {
		checkPathTags(t, typ)
	}
}

// checkPathTags 检查一个结构体（包括内嵌的结构体）里每个带 uri 标签的字段。
func checkPathTags(t *testing.T, typ reflect.Type) {
	t.Helper()
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.Anonymous {
			checkPathTags(t, field.Type)
			continue
		}
		if field.Tag.Get("uri") == "" {
			continue
		}
		if field.Tag.Get("json") != "-" || field.Tag.Get("form") != "-" {
			t.Errorf("%s.%s 是路径参数，必须同时标 json:\"-\" 和 form:\"-\"，实际标签 %q", typ.Name(), field.Name, field.Tag)
		}
	}
}
