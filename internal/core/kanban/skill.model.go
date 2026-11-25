package kanban

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"slate-kanban/skills"
)

// Skill 是服务级自定义 Skill。内置 Skill 随程序发布，不写入数据库。
type Skill struct {
	Name        string    `gorm:"primaryKey;size:64;comment:Skill 名称"`
	Description string    `gorm:"type:text;notNull;comment:Skill 简介"`
	Markdown    string    `gorm:"type:text;notNull;comment:完整 SKILL.md"`
	Digest      string    `gorm:"size:71;notNull;comment:内容摘要"`
	Size        int       `gorm:"notNull;comment:文件字节数"`
	CreatedAt   time.Time `gorm:"notNull;default:CURRENT_TIMESTAMP;comment:创建时间"`
	UpdatedAt   time.Time `gorm:"notNull;default:CURRENT_TIMESTAMP;comment:更新时间"`
}

func (s Skill) summary() SkillSummary {
	return SkillSummary{Name: s.Name, Description: s.Description, URI: skills.URI(s.Name), Digest: s.Digest, Size: s.Size, UpdatedAt: s.UpdatedAt}
}

func (s Skill) detail() SkillDetail {
	_, body, _ := strings.Cut(s.Markdown, "\n---\n\n")
	return SkillDetail{SkillSummary: s.summary(), Body: body, markdown: s.Markdown}
}

// newSkill 只在写入时生成并保存完整 Markdown 和 manifest，列表无需从数据库读取正文。
func newSkill(in SkillInput, now time.Time) *Skill {
	detail := skillDetail(skills.Document{Name: in.Name, Description: in.Description, Body: in.Body}, false, now)
	return &Skill{Name: in.Name, Description: in.Description, Markdown: detail.Markdown(), Digest: detail.Digest, Size: detail.Size, CreatedAt: now, UpdatedAt: now}
}

// SkillInput 是创建、完整编辑 Skill 的入参。
type SkillInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
}

// SkillSummary 是目录元数据，包含内容的摘要和原始字节数，不返回正文。
type SkillSummary struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	URI         string    `json:"uri"`
	Digest      string    `json:"digest"`
	Size        int       `json:"size"`
	Builtin     bool      `json:"builtin"`
	UpdatedAt   time.Time `json:"updated_at,omitzero"`
}

// SkillDetail 是读取和写入后的完整 Skill。
type SkillDetail struct {
	SkillSummary
	Body     string `json:"body"`
	markdown string
}

// Markdown 返回与目录元数据对应的 SKILL.md 原文。
func (s SkillDetail) Markdown() string {
	return s.markdown
}

func skillDetail(doc skills.Document, builtin bool, updated time.Time) SkillDetail {
	markdown := doc.Markdown()
	return SkillDetail{SkillSummary: SkillSummary{
		Name: doc.Name, Description: doc.Description, URI: doc.URI(),
		Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(markdown))), Size: len(markdown),
		Builtin: builtin, UpdatedAt: updated,
	}, Body: doc.Body, markdown: markdown}
}
