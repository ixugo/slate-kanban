package mcpapi_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// pngBytes 是一张合法 PNG 的开头：8 字节签名加一点内容，足够让按内容判断类型的逻辑认出 PNG。
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), []byte("0000slate-test-image")...)

// writeImage 把图片字节写到临时目录里，返回绝对路径。
func writeImage(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("写临时文件失败: %v", err)
	}
	return p
}

// callImage 调用 get_image 并返回原始结果，图片内容不会出现在结构化结果里。
func callImage(t *testing.T, sess *mcp.ClientSession, token string) *mcp.CallToolResult {
	t.Helper()
	res, err := sess.CallTool(t.Context(), &mcp.CallToolParams{Name: "get_image", Arguments: map[string]any{"token": token}})
	if err != nil {
		t.Fatalf("调用 get_image 失败: %v", err)
	}
	return res
}

// TestUploadImageThenReference 验证完整闭环：用本机路径上传，把返回的 markdown 写进任务描述，
// 之后用编号和整个地址都能读回同样的图片。
func TestUploadImageThenReference(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")

	up := ok(t, sess, "upload_image", map[string]any{"board_id": b.ID, "path": writeImage(t, "shot.png", pngBytes)}).Data
	token, _ := up["token"].(string)
	md, _ := up["markdown"].(string)
	if len(token) != 32 || up["content_type"] != "image/png" || up["size"] != float64(len(pngBytes)) ||
		md != "![](/api/v1/attachments/"+token+")" {
		t.Fatalf("上传结果不对: %v", up)
	}
	created := ok(t, sess, "create_card", map[string]any{"board_id": b.ID, "title": "带图", "column_id": b.todo, "description": "截图：" + md}).Data
	card := ok(t, sess, "get_card", map[string]any{"card_id": created["id"]}).Data
	if desc, _ := card["description"].(string); !strings.Contains(desc, token) {
		t.Fatalf("描述里应该带着图片: %v", card)
	}

	for _, ref := range []string{token, "/api/v1/attachments/" + token} {
		res := callImage(t, sess, ref)
		img, isImg := res.Content[0].(*mcp.ImageContent)
		if res.IsError || !isImg || string(img.Data) != string(pngBytes) || img.MIMEType != "image/png" {
			t.Fatalf("按 %q 读回的图片不对: %+v", ref, res)
		}
	}
}

// TestUploadImageBase64 验证 base64 方式能上传，人的管理视图也有这两个工具。
func TestUploadImageBase64(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	up := ok(t, s.human(t), "upload_image", map[string]any{
		"board_id": b.ID, "data_base64": base64.StdEncoding.EncodeToString(pngBytes),
	}).Data
	if up["content_type"] != "image/png" || len(up["token"].(string)) != 32 {
		t.Fatalf("base64 上传结果不对: %v", up)
	}
	if res := callImage(t, s.human(t), up["token"].(string)); res.IsError {
		t.Fatalf("管理视图应该能读到图片: %+v", res)
	}
}

// TestUploadImageRejectsBadInput 验证各种不合法的入参都被拒绝，并且错误说明没有泄露服务器上的文件信息。
func TestUploadImageRejectsBadInput(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	good := writeImage(t, "ok.png", pngBytes)
	text := writeImage(t, "note.txt", []byte("这不是图片"))
	big := writeImage(t, "big.png", append(append([]byte{}, pngBytes...), make([]byte, 5<<20)...))
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"两个都填", map[string]any{"board_id": b.ID, "path": good, "data_base64": "AAAA"}, "只能填一个"},
		{"两个都不填", map[string]any{"board_id": b.ID}, "请填 path 或 data_base64"},
		{"相对路径", map[string]any{"board_id": b.ID, "path": "shot.png"}, "绝对路径"},
		{"文件不存在", map[string]any{"board_id": b.ID, "path": filepath.Join(t.TempDir(), "none.png")}, "读不到"},
		{"路径是目录", map[string]any{"board_id": b.ID, "path": t.TempDir()}, "读不到"},
		{"不是图片", map[string]any{"board_id": b.ID, "path": text}, "只支持 PNG"},
		{"base64 非法", map[string]any{"board_id": b.ID, "data_base64": "***"}, "base64"},
		{"文件过大", map[string]any{"board_id": b.ID, "path": big}, "图片太大"},
		{"看板不存在", map[string]any{"board_id": 99999, "path": good}, "看板不存在"},
	}
	for _, tc := range cases {
		out := call(t, sess, "upload_image", tc.args)
		if !out.IsError || !strings.Contains(out.Text, tc.want) {
			t.Errorf("%s：应该报错并包含 %q，实际: %+v", tc.name, tc.want, out)
		}
		if strings.Contains(out.Text, filepath.Dir(good)) {
			t.Errorf("%s：错误说明不应该带出服务器上的路径: %s", tc.name, out.Text)
		}
	}
}

// TestGetImageUnknownToken 验证编号格式不对或图片不存在时返回工具错误。
func TestGetImageUnknownToken(t *testing.T) {
	sess := newStack(t).agent(t, "claude")
	for _, token := range []string{"", "abc", strings.Repeat("a", 32), "/api/v1/attachments/../../etc/passwd"} {
		if res := callImage(t, sess, token); !res.IsError {
			t.Errorf("token %q 应该报错: %+v", token, res)
		}
	}
}
