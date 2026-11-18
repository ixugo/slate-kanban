package svcinstall

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// testInput 构造一份用于生成服务文件内容的固定输入，方便断言输出里是否包含关键字段。
func testInput(exe, conf, work string) ServiceInput {
	return ServiceInput{
		Executable: exe,
		ConfDir:    conf,
		WorkDir:    work,
	}
}

// lineWithPrefix 取出文本里第一行以 prefix 开头的整行，没有就让测试失败。
func lineWithPrefix(t *testing.T, text, prefix string) string {
	t.Helper()
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, prefix) {
			return l
		}
	}
	t.Fatalf("找不到以 %q 开头的行:\n%s", prefix, text)
	return ""
}

// plistStrings 用 XML 解析器读 plist，按出现顺序返回所有 <string> 的文字；解析失败说明 plist 被写坏了。
func plistStrings(t *testing.T, plist string) []string {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(plist))
	var out []string
	inString := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatalf("plist 不是合法的 XML: %v\n%s", err, plist)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			inString = v.Name.Local == "string"
		case xml.EndElement:
			inString = false
		case xml.CharData:
			if inString {
				out = append(out, string(v))
			}
		}
	}
}

// TestLaunchdContentNormalPath 普通路径下 launchd plist 应包含可执行文件、配置目录、工作目录、开机加载和保活设置。
func TestLaunchdContentNormalPath(t *testing.T) {
	in := testInput("/usr/local/bin/slate", "/home/user/proj/configs", "/home/user/proj")
	got := GenerateLaunchdPlist(in)
	want := []string{
		"dev.slate.kanban",
		"/usr/local/bin/slate",
		"-conf",
		"/home/user/proj/configs",
		"/home/user/proj",
		"/home/user/proj/logs/slate-service.stdout.log",
		"/home/user/proj/logs/slate-service.stderr.log",
	}
	if gotStrings := plistStrings(t, got); strings.Join(gotStrings, "\n") != strings.Join(want, "\n") {
		t.Fatalf("plist 里的字符串不对\nwant: %q\ngot:  %q", want, gotStrings)
	}
	for _, key := range []string{"<key>RunAtLoad</key>", "<key>KeepAlive</key>", "<key>WorkingDirectory</key>"} {
		if !strings.Contains(got, key) {
			t.Fatalf("plist 缺少 %s\n%s", key, got)
		}
	}
}

// TestLaunchdContentSpecialChars 路径里的 & < > 引号必须转义，XML 解析回来要和原路径一模一样。
func TestLaunchdContentSpecialChars(t *testing.T) {
	exe := "/Apps/Slate & Co/slate"
	conf := `/home/user/my "proj"/configs`
	work := "/home/user/my <proj>'s"
	got := plistStrings(t, GenerateLaunchdPlist(testInput(exe, conf, work)))
	want := []string{"dev.slate.kanban", exe, "-conf", conf, work, work + "/logs/slate-service.stdout.log", work + "/logs/slate-service.stderr.log"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("特殊字符没有原样还原\nwant: %q\ngot:  %q", want, got)
	}
}

// TestSystemdContentNormalPath 普通路径下 systemd 单元的关键行要完全符合预期，路径不加引号。
func TestSystemdContentNormalPath(t *testing.T) {
	got := GenerateSystemdUnit(testInput("/usr/local/bin/slate", "/home/user/proj/configs", "/home/user/proj"))
	for _, want := range []string{
		"WorkingDirectory=/home/user/proj",
		"ExecStart=/usr/local/bin/slate -conf /home/user/proj/configs",
		"Restart=on-failure",
		"WantedBy=default.target",
	} {
		if !strings.Contains("\n"+got, "\n"+want+"\n") {
			t.Fatalf("缺少或写错了 %q\n%s", want, got)
		}
	}
}

// TestSystemdContentSpecialChars 空格、引号、反斜杠、% 和 $ 各自按 systemd 规则处理：
// ExecStart 的参数加引号并转义，WorkingDirectory 只转义 %，空格原样保留不加引号。
func TestSystemdContentSpecialChars(t *testing.T) {
	cases := []struct {
		name, exe, conf, work, wantExec, wantWork string
	}{
		{
			name:     "空格",
			exe:      "/Apps/Slate Co/slate",
			conf:     "/home/u/my proj/configs",
			work:     "/home/u/my proj",
			wantExec: `ExecStart="/Apps/Slate Co/slate" -conf "/home/u/my proj/configs"`,
			wantWork: "WorkingDirectory=/home/u/my proj",
		},
		{
			name:     "百分号和美元符",
			exe:      "/opt/100%/a$b/slate",
			conf:     "/opt/100%/configs",
			work:     "/opt/100%",
			wantExec: `ExecStart="/opt/100%%/a$$b/slate" -conf "/opt/100%%/configs"`,
			wantWork: "WorkingDirectory=/opt/100%%",
		},
		{
			name:     "引号和反斜杠",
			exe:      `/opt/a"b\c/slate`,
			conf:     "/etc/slate",
			work:     "/opt",
			wantExec: `ExecStart="/opt/a\"b\\c/slate" -conf /etc/slate`,
			wantWork: "WorkingDirectory=/opt",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GenerateSystemdUnit(testInput(c.exe, c.conf, c.work))
			if line := lineWithPrefix(t, got, "ExecStart="); line != c.wantExec {
				t.Fatalf("ExecStart 不对\nwant: %s\ngot:  %s", c.wantExec, line)
			}
			if line := lineWithPrefix(t, got, "WorkingDirectory="); line != c.wantWork {
				t.Fatalf("WorkingDirectory 不对\nwant: %s\ngot:  %s", c.wantWork, line)
			}
		})
	}
}
