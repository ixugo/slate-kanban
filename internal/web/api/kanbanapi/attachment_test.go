package kanbanapi_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/web/api/apitest"
)

// pngFile 是以 PNG 文件头开头的假图片，类型嗅探只看文件头。
var pngFile = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)

// upload 以表单方式上传文件，field 是表单字段名。
func (s *stack) upload(t *testing.T, boardID int64, field string, data []byte) apitest.Response {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile(field, "shot.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/boards/"+id(boardID)+"/attachments", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return s.Serve(req)
}

// TestAttachmentUploadAndRead 验证上传后拿到地址，按地址读回的字节和响应头都对。
func TestAttachmentUploadAndRead(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "IMG")
	out := decode[struct {
		Token string `json:"token"`
		URL   string `json:"url"`
		Type  string `json:"content_type"`
		Size  int64  `json:"size"`
	}](t, s.upload(t, b.ID, "file", pngFile))
	if !strings.HasPrefix(out.URL, "/api/v1/attachments/") || out.Type != "image/png" || out.Size != int64(len(pngFile)) {
		t.Fatalf("上传结果不对: %+v", out)
	}

	got := s.human(t, http.MethodGet, out.URL[len("/api/v1"):], nil)
	if got.Code != http.StatusOK || !bytes.Equal(got.Body, pngFile) {
		t.Fatalf("读回不对: %d", got.Code)
	}
	h := got.Headers
	if h.Get("Content-Type") != "image/png" || h.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(h.Get("Content-Security-Policy"), "sandbox") || h.Get("Content-Disposition") != "inline" {
		t.Fatalf("安全响应头不对: %v", h)
	}
}

// TestAttachmentRejects 验证非法上传都得到明确的错误：缺字段、非图片、超大、看板不存在、编号不对。
func TestAttachmentRejects(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "IMG")
	wantFail(t, "字段名不对", s.upload(t, b.ID, "image", pngFile), http.StatusBadRequest, "ErrInvalidInput")
	wantFail(t, "不是图片", s.upload(t, b.ID, "file", []byte("<svg><script>alert(1)</script></svg>")), http.StatusBadRequest, "ErrInvalidInput")
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, kanban.MaxAttachmentBytes)...)
	wantFail(t, "超大", s.upload(t, b.ID, "file", big), http.StatusRequestEntityTooLarge, "ErrAttachmentTooLarge")
	wantFail(t, "看板不存在", s.upload(t, b.ID+99, "file", pngFile), http.StatusNotFound, "ErrBoardNotFound")
	wantFail(t, "编号不对", s.human(t, http.MethodGet, "/attachments/not-a-token", nil), http.StatusNotFound, "ErrAttachmentNotFound")
}
