package kanban_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/core/kanban/stores/kanbandb"
	"slate-kanban/skills"
)

func TestSkillPersistenceAndMigration(t *testing.T) {
	kanbantest.ForEachRawBackend(t, func(t *testing.T, raw *gorm.DB) {
		ctx := t.Context()
		db := kanbandb.NewDB(raw).AutoMigrate(true)
		core := kanban.NewCore(db)
		human := kanban.Actor{Kind: kanban.ActorHuman, Name: "admin"}
		board, err := core.CreateBoard(ctx, human, kanban.CreateBoardInput{Name: "Existing", Slug: "OLD"})
		if err != nil {
			t.Fatal(err)
		}
		// 用隔离测试库模拟尚未发布 Skill 功能的旧数据库，并确认启动迁移不会动现有项目。
		if err := raw.Migrator().DropTable(new(kanban.Skill)); err != nil {
			t.Fatal(err)
		}
		db.AutoMigrate(true)
		in := kanban.SkillInput{Name: "true", Description: `适用时机: "测试" # 保留`, Body: "\n# Markdown\n\n    保留缩进\n---\n"}
		created, err := core.CreateSkill(ctx, human, in)
		if err != nil {
			t.Fatal(err)
		}
		if created.Body != in.Body || created.Size != len(created.Markdown()) || created.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(created.Markdown()))) {
			t.Fatalf("内容或 manifest 不一致: %+v", created)
		}
		db.AutoMigrate(true)
		restarted := kanban.NewCore(db)
		if got, err := restarted.GetBoard(ctx, board.ID); err != nil || got.Name != "Existing" {
			t.Fatalf("旧项目被修改: %+v %v", got, err)
		}
		if got, err := restarted.GetSkill(ctx, in.Name); err != nil || got.Markdown() != created.Markdown() || got.Digest != created.Digest {
			t.Fatalf("Skill 未持久化: %+v %v", got, err)
		}
		metadata, err := db.Skill().ListMetadata(ctx)
		if err != nil || len(metadata) != 1 || metadata[0].Markdown != "" || metadata[0].Digest != created.Digest || metadata[0].Size != created.Size {
			t.Fatalf("目录读取不应载入正文: %+v %v", metadata, err)
		}
		if got, err := db.Skill().GetMetadata(ctx, in.Name); err != nil || got.Markdown != "" || got.Digest != created.Digest {
			t.Fatalf("单项元数据读取不应载入正文: %+v %v", got, err)
		}
		in.Body = "new body"
		updated, err := restarted.UpdateSkill(ctx, human, in.Name, in)
		if err != nil || updated.Digest == created.Digest || updated.Body != in.Body {
			t.Fatalf("更新未同步 manifest: %+v %v", updated, err)
		}
		if err := restarted.DeleteSkill(ctx, human, in.Name); err != nil {
			t.Fatal(err)
		}
		if err := restarted.DeleteSkill(ctx, human, in.Name); err != nil {
			t.Fatalf("重复删除应幂等: %v", err)
		}
		if _, err := restarted.GetSkill(ctx, in.Name); !errors.Is(err, kanban.ErrSkillNotFound) {
			t.Fatalf("删除后仍能读取: %v", err)
		}
	})
}

func TestSkillValidationAndPermissions(t *testing.T) {
	core := kanbantest.NewCore(t)
	ctx := t.Context()
	human := kanban.Actor{Kind: kanban.ActorHuman, Name: "admin"}
	agent := kanban.Actor{Kind: kanban.ActorAgent, Name: "worker"}
	valid := kanban.SkillInput{Name: "custom-skill", Description: "description", Body: "# body"}
	for _, name := range []string{"", "UPPER", "a--b", "-a", "a-", "a/b", "a_b", strings.Repeat("a", 65)} {
		in := valid
		in.Name = name
		if _, err := core.CreateSkill(ctx, human, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("非法名称 %q 应拒绝: %v", name, err)
		}
	}
	for _, desc := range []string{"", "  ", "a\nb", strings.Repeat("中", 1025)} {
		in := valid
		in.Description = desc
		if _, err := core.CreateSkill(ctx, human, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Error("非法简介应拒绝", err)
		}
	}
	for _, body := range []string{"", " \t\n", "a\x00b", string([]byte{0xff}), strings.Repeat("中", kanban.MaxSkillBodyBytes/3+1)} {
		in := valid
		in.Body = body
		if _, err := core.CreateSkill(ctx, human, in); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Error("非法正文应拒绝", err)
		}
	}
	if _, err := core.CreateSkill(ctx, agent, valid); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Errorf("Agent 不可创建 Skill: %v", err)
	}
	if _, err := core.CreateSkill(ctx, human, valid); err != nil {
		t.Fatal(err)
	}
	if _, err := core.UpdateSkill(ctx, agent, valid.Name, valid); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Errorf("Agent 不可修改 Skill: %v", err)
	}
	if err := core.DeleteSkill(ctx, agent, valid.Name); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Errorf("Agent 不可删除 Skill: %v", err)
	}
	other := valid
	other.Name = "other-guide"
	if _, err := core.CreateSkill(ctx, human, other); err != nil {
		t.Fatal(err)
	}
	if _, err := core.UpdateSkill(ctx, human, valid.Name, other); !errors.Is(err, kanban.ErrSkillNameTaken) {
		t.Fatalf("不能重命名覆盖其他自定义 Skill: %v", err)
	}
	if got, err := core.GetSkill(ctx, valid.Name); err != nil || got.Body != valid.Body {
		t.Fatalf("失败更新不应改动旧 Skill: %+v %v", got, err)
	}

	for _, doc := range skills.Builtins() {
		in := valid
		in.Name = doc.Name
		if _, err := core.CreateSkill(ctx, human, in); !errors.Is(err, kanban.ErrSkillNameTaken) {
			t.Errorf("不可创建同名内置 Skill %s: %v", doc.Name, err)
		}
		if _, err := core.UpdateSkill(ctx, human, valid.Name, in); !errors.Is(err, kanban.ErrSkillNameTaken) {
			t.Errorf("不可重命名覆盖内置 Skill %s: %v", doc.Name, err)
		}
		if _, err := core.UpdateSkill(ctx, human, doc.Name, valid); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("内置 Skill 不可修改 %s: %v", doc.Name, err)
		}
		if err := core.DeleteSkill(ctx, human, doc.Name); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("内置 Skill 不可删除 %s: %v", doc.Name, err)
		}
	}
}
