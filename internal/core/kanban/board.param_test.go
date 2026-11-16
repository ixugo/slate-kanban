package kanban

import (
	"errors"
	"strings"
	"testing"
)

// TestValidateSlug 验证看板标识的格式：长度、首位必须是大写字母、只能有大写字母和数字。
func TestValidateSlug(t *testing.T) {
	good := []string{"AB", "KANBAN", "A1", "ABCDEFGHIJ", "P2024"}
	bad := []string{"", "A", "ABCDEFGHIJK", "1ABC", "ab", "A-B", "A B", "AB_C", "中文", "A\x00"}
	for _, s := range good {
		if err := validateSlug(s); err != nil {
			t.Errorf("%q 应该合法: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := validateSlug(s); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%q 应该被拒绝并返回 ErrInvalidInput，实际 %v", s, err)
		}
	}
}

// TestGenerateSlug 验证由名称生成标识：取前 6 位字母数字转大写，纯中文或数字开头用兜底值，
// 冲突后加数字后缀，并且无论怎么冲突，结果都是合法标识。
func TestGenerateSlug(t *testing.T) {
	cases := []struct {
		name    string
		attempt int
		want    string
	}{
		{"My Project", 0, "MYPROJ"},
		{"slate-kanban", 0, "SLATEK"},
		{"看板", 0, "BOARD"},
		{"2024 plan", 0, "BOARD"},
		{"a", 0, "BOARD"},
		{"ab", 0, "AB"},
		{"MY PROJECT", 1, "MYPROJ2"},
		{"My Project", 8, "MYPROJ9"},
		{"My Project", 9, "MYPROJ10"},
		{"看板", 1, "BOARD2"},
	}
	for _, c := range cases {
		if got := generateSlug(c.name, c.attempt); got != c.want {
			t.Errorf("generateSlug(%q, %d) = %q，期望 %q", c.name, c.attempt, got, c.want)
		}
	}
	for _, name := range []string{"My Project", "看板", "ab", "x y z 9"} {
		for attempt := range maxSlugAttempts {
			if err := validateSlug(generateSlug(name, attempt)); err != nil {
				t.Errorf("generateSlug(%q, %d) 生成了不合法的标识: %v", name, attempt, err)
			}
		}
	}
}

// TestValidateProjectPath 验证项目路径：可以为空，非空必须是 Unix、Windows 盘符或网络共享的绝对路径。
func TestValidateProjectPath(t *testing.T) {
	good := []string{"", "/Users/me/code", "/", `C:\code\app`, "d:/code", `\\server\share\app`}
	bad := []string{"code/app", "./app", "~/app", `C:app`, "1:/app", "/a\nb", "/a\x00b", "/" + strings.Repeat("a", maxProjectPathLen)}
	for _, p := range good {
		if err := validateProjectPath(p); err != nil {
			t.Errorf("%q 应该合法: %v", p, err)
		}
	}
	for _, p := range bad {
		if err := validateProjectPath(p); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%q 应该被拒绝并返回 ErrInvalidInput，实际 %v", p, err)
		}
	}
}

// TestCreateBoardInputNormalize 验证创建入参：整理空白、slug 转大写、模板缺省为 dev，
// 不合法的字段返回 ErrInvalidInput。
func TestCreateBoardInputNormalize(t *testing.T) {
	in := CreateBoardInput{Name: "  我的项目  ", Slug: " proj1 ", ProjectPath: " /code ", Template: " "}
	if err := in.normalize(); err != nil {
		t.Fatalf("合法入参不应该报错: %v", err)
	}
	if in.Name != "我的项目" || in.Slug != "PROJ1" || in.ProjectPath != "/code" || in.Template != TemplateDev {
		t.Fatalf("整理结果不对: %+v", in)
	}

	bad := []CreateBoardInput{
		{Name: "   "},
		{Name: strings.Repeat("长", maxBoardNameLen+1)},
		{Name: "a", Description: strings.Repeat("长", maxBoardDescLen+1)},
		{Name: "a", Slug: "1bad"},
		{Name: "a", ProjectPath: "relative"},
		{Name: "a", Template: "kanban"},
	}
	for i, in := range bad {
		if err := in.normalize(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("第 %d 组应该返回 ErrInvalidInput，实际 %v", i, err)
		}
	}
}

// TestUpdateBoardInputNormalize 验证修改入参只检查出现的字段，整理空白，
// 并且不会改动调用方原来的字符串。
func TestUpdateBoardInputNormalize(t *testing.T) {
	name, path := "  新名字 ", " /code "
	in := UpdateBoardInput{Name: &name, ProjectPath: &path}
	if err := in.normalize(); err != nil {
		t.Fatalf("合法入参不应该报错: %v", err)
	}
	if *in.Name != "新名字" || *in.ProjectPath != "/code" || in.Description != nil {
		t.Fatalf("整理结果不对: %q %q %v", *in.Name, *in.ProjectPath, in.Description)
	}
	if name != "  新名字 " {
		t.Fatalf("不应该改动调用方原来的字符串: %q", name)
	}

	empty, rel := "   ", "relative"
	for _, in := range []UpdateBoardInput{{Name: &empty}, {ProjectPath: &rel}} {
		if err := in.normalize(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("应该返回 ErrInvalidInput，实际 %v", err)
		}
	}
	blank := ""
	in = UpdateBoardInput{ProjectPath: &blank}
	if err := in.normalize(); err != nil {
		t.Fatalf("允许把项目路径清空: %v", err)
	}
}
