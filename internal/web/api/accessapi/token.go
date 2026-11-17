package accessapi

import (
	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/access"
)

// createTokenInput 是创建令牌的请求体。
type createTokenInput struct {
	Name string `json:"name"`
}

// tokenIDInput 是路径里的令牌编号，只从 URL 取，请求体和查询参数覆盖不了。
type tokenIDInput struct {
	TokenID int64 `uri:"token_id" json:"-" form:"-"`
}

// empty 是没有内容的成功响应，序列化成 {}。
type empty struct{}

// createToken 创建 agent 令牌，返回的明文只此一次，之后无法找回。只有管理令牌能调用，由 Core 判断。
func (a API) createToken(c *gin.Context, in *createTokenInput) (*access.NewToken, error) {
	p, err := PrincipalFrom(c.Request.Context())
	if err != nil {
		return nil, err
	}
	return a.core.CreateToken(c.Request.Context(), p, in.Name)
}

// listTokens 列出全部 agent 令牌，不含哈希和明文。
func (a API) listTokens(c *gin.Context, _ *struct{}) ([]*access.Token, error) {
	p, err := PrincipalFrom(c.Request.Context())
	if err != nil {
		return nil, err
	}
	return a.core.ListTokens(c.Request.Context(), p)
}

// deleteToken 删除一个令牌，删除后持有它的 agent 立刻得到 401；令牌不存在也返回成功。
func (a API) deleteToken(c *gin.Context, in *tokenIDInput) (empty, error) {
	p, err := PrincipalFrom(c.Request.Context())
	if err != nil {
		return empty{}, err
	}
	return empty{}, a.core.DeleteToken(c.Request.Context(), p, in.TokenID)
}
