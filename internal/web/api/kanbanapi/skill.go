package kanbanapi

import (
	"context"

	"github.com/gin-gonic/gin"

	"slate-kanban/internal/core/kanban"
)

func (a API) skillRoutes(r gin.IRouter) {
	r.GET("/skills", route(a.listSkills))
	r.POST("/skills", route(a.createSkill))
	r.GET("/skills/:skill_name", route(a.getSkill))
	r.PUT("/skills/:skill_name", route(a.updateSkill))
	r.DELETE("/skills/:skill_name", route(a.deleteSkill))
}

type skillPath struct {
	SkillName string `uri:"skill_name" json:"-" form:"-"`
}

type updateSkillInput struct {
	skillPath
	kanban.SkillInput
}

// listSkills 返回全局 Skill 列表（不含正文）。
func (a API) listSkills(ctx context.Context, _ kanban.Actor, _ *struct{}) ([]kanban.SkillSummary, error) {
	return a.core.ListSkills(ctx)
}

// getSkill 读取指定 Skill 的完整内容。
func (a API) getSkill(ctx context.Context, _ kanban.Actor, in *skillPath) (*kanban.SkillDetail, error) {
	return a.core.GetSkill(ctx, in.SkillName)
}

// createSkill 创建自定义 Skill。
func (a API) createSkill(ctx context.Context, actor kanban.Actor, in *kanban.SkillInput) (*kanban.SkillDetail, error) {
	return a.core.CreateSkill(ctx, actor, *in)
}

// updateSkill 完整替换自定义 Skill。
func (a API) updateSkill(ctx context.Context, actor kanban.Actor, in *updateSkillInput) (*kanban.SkillDetail, error) {
	return a.core.UpdateSkill(ctx, actor, in.SkillName, in.SkillInput)
}

// deleteSkill 删除自定义 Skill。
func (a API) deleteSkill(ctx context.Context, actor kanban.Actor, in *skillPath) (empty, error) {
	return empty{}, a.core.DeleteSkill(ctx, actor, in.SkillName)
}
