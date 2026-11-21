package kanban

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"
)

// ProjectTemplateStorer 是自定义项目模板的数据持久化接口。
type ProjectTemplateStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) ProjectTemplateStorer
	// Create 新增一条模板。名称重复时返回可用 orm.IsDuplicatedKey 判断的错误。
	Create(context.Context, *ProjectTemplate) error
	// Delete 按主键删除，幂等：不存在也返回成功。
	Delete(context.Context, int64) error
	// GetByID 按主键读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByID(context.Context, int64) (*ProjectTemplate, error)
	// GetByName 按名称读取，区分大小写；不存在时错误同 GetByID。
	GetByName(context.Context, string) (*ProjectTemplate, error)
	// List 返回全部自定义模板，按名称升序。
	List(context.Context) ([]*ProjectTemplate, error)
}
