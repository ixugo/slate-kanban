package mcpapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

const (
	// maxImagePathLen 是 upload_image 的 path 长度上限，超过操作系统的路径上限没有意义。
	maxImagePathLen = 4096
)

// uploadImageIn 是 upload_image 的入参，path 和 data_base64 二选一。
type uploadImageIn struct {
	BoardID    int64  `json:"board_id" jsonschema:"图片要用在哪个看板，来自 list_boards"`
	Path       string `json:"path,omitempty" jsonschema:"图片文件在运行 Slate 的这台机器上的绝对路径；和 data_base64 二选一"`
	DataBase64 string `json:"data_base64,omitempty" jsonschema:"图片内容的 base64 编码；和 path 二选一；整个请求不能超过 1 MiB，只适合很小的图片，一般用 path"`
}

// getImageIn 是 get_image 的入参。
type getImageIn struct {
	Token string `json:"token" jsonschema:"图片编号：描述或评论里图片地址 /api/v1/attachments/ 后面的 32 位字符，也可以直接填整个图片地址"`
}

// uploadImageOut 是 upload_image 的结果：markdown 可以原样写进任务描述或评论。
type uploadImageOut struct {
	Token       string `json:"token"`
	URL         string `json:"url"`
	Markdown    string `json:"markdown"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// registerImageTools 注册图片的上传和查看。两个视图都有：agent 要给任务配截图，也要看人贴在描述和评论里的图。
func (a API) registerImageTools(s *mcp.Server) {
	register(s, spec{
		name: "upload_image", kind: kindWrite,
		desc: "上传一张图片，返回可以直接写进任务描述或评论的 markdown。只收 PNG、JPEG、GIF、WebP，最大 5 MB。" +
			"优先用 path 传本机文件的绝对路径（Slate 和你在同一台机器上才可用）；拿不到文件时用 data_base64（只适合很小的图片，整个请求不能超过 1 MiB）。" +
			"图片要等引用它的描述或评论保存之后才会留在看板里：上传后请把返回的 markdown 写进 create_card、update_card 或 comment_card，" +
			"没被引用的图片不会留在看板里。",
	}, a.uploadImage)
	a.registerGetImage(s)
}

// registerGetImage 注册 get_image。结果是 MCP 的图片内容而不是 JSON，所以不走 register 的包装，
// 身份校验和错误转换沿用同样的做法。
func (a API) registerGetImage(s *mcp.Server) {
	tool := &mcp.Tool{
		Name: "get_image",
		Description: "按图片编号读取一张图片并返回图片本身，用来查看任务描述或评论里的截图。" +
			"描述里形如 ![](/api/v1/attachments/<32 位编号>) 的就是本看板的图片。",
		Annotations: kindRead.annotations(),
	}
	mcp.AddTool(s, tool, func(ctx context.Context, req *mcp.CallToolRequest, in getImageIn) (*mcp.CallToolResult, any, error) {
		if _, err := requestActor(req); err != nil {
			return nil, nil, toolError(ctx, err)
		}
		item, data, err := a.core.GetAttachment(ctx, tokenOf(in.Token))
		if err != nil {
			return nil, nil, toolError(ctx, err)
		}
		note := fmt.Sprintf("%s，%d 字节", item.ContentType, item.Size)
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.ImageContent{Data: data, MIMEType: item.ContentType},
			&mcp.TextContent{Text: note},
		}}, nil, nil
	})
}

// tokenOf 从入参里取出图片编号：填整个图片地址时取最后一段，其余原样交给 Core 校验格式。
func tokenOf(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// uploadImage 把本机文件或 base64 内容存成一张待引用的图片。类型、大小、看板是否存在都由 Core 判断。
func (a API) uploadImage(ctx context.Context, actor kanban.Actor, in uploadImageIn) (any, error) {
	data, err := imageBytes(in)
	if err != nil {
		return nil, err
	}
	item, err := a.core.SaveAttachment(ctx, actor, in.BoardID, data)
	if err != nil {
		return nil, err
	}
	url := "/api/v1/attachments/" + item.Token
	return uploadImageOut{
		Token: item.Token, URL: url, Markdown: "![](" + url + ")", ContentType: item.ContentType, Size: item.Size,
	}, nil
}

// imageBytes 按入参读出图片字节：path 和 data_base64 必须恰好填一个。
func imageBytes(in uploadImageIn) ([]byte, error) {
	switch {
	case in.Path != "" && in.DataBase64 != "":
		return nil, kanban.ErrInvalidInput.WithMsg("path 和 data_base64 只能填一个")
	case in.Path != "":
		return readImageFile(in.Path)
	case in.DataBase64 != "":
		return decodeImage(in.DataBase64)
	}
	return nil, kanban.ErrInvalidInput.WithMsg("请填 path 或 data_base64")
}

// readImageFile 读本机的一个普通文件，最多读上限加一个字节：超出的部分交给 Core 判定为太大，
// 不会把一个巨大的文件整个读进内存。读到的内容是不是图片由 Core 按内容判断，所以不是图片的文件进不了看板。
func readImageFile(path string) ([]byte, error) {
	if len(path) > maxImagePathLen || !filepath.IsAbs(path) {
		return nil, kanban.ErrInvalidInput.WithMsg("path 必须是运行 Slate 的这台机器上的绝对路径")
	}
	unreadable := kanban.ErrInvalidInput.WithMsg("读不到这个路径上的图片文件，请确认它存在、是普通文件并且 Slate 有权读取")
	f, err := os.Open(path)
	if err != nil {
		return nil, unreadable
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		return nil, unreadable
	}
	data, err := io.ReadAll(io.LimitReader(f, kanban.MaxAttachmentBytes+1))
	if err != nil {
		return nil, unreadable
	}
	return data, nil
}

// decodeImage 解开 base64 内容。整个 MCP 请求体有 1 MiB 的上限，所以进到这里的字符串不会太大。
func decodeImage(s string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, kanban.ErrInvalidInput.WithMsg("data_base64 不是合法的 base64 内容")
	}
	return data, nil
}
