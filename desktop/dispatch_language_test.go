package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// 界面切换语言后，下一次派发的规划、研发和审查都应要求宿主按该语言输出，
// 桌面端追加的提交说明和任务项标题也应使用同一种语言。
func TestDesktopDispatchUsesWebViewLanguage(t *testing.T) {
	core, human, board, flow := newDispatchTestBoard(t)
	lang := newDesktopLanguage(t.TempDir())
	const origin = "http://127.0.0.1:7070"
	handler := lang.wrap(origin, http.NotFoundHandler())
	run := dispatchRun{core: core, sessions: t.TempDir(), text: lang.text}
	for _, tt := range []struct {
		code, language, heading, commitFormat string
		plan, summary, review                 string
	}{
		{"en", "English", "Task items", "Commit: `%s`.", "Development plan", "Implemented", "Review passed"},
		{"zh-CN", "Simplified Chinese (简体中文)", "任务项", "提交：`%s`。", "研发方案", "已实现", "审查通过"},
		{"zh-TW", "Traditional Chinese (繁體中文)", "任務項", "提交：`%s`。", "研發方案", "已實作", "審查通過"},
		{"ja", "Japanese (日本語)", "タスク項目", "コミット：`%s`。", "開発計画", "実装済み", "レビュー合格"},
	} {
		t.Run(tt.code, func(t *testing.T) {
			ctx := t.Context()
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, origin+lang.path, strings.NewReader(tt.code))
			req.Header.Set("Origin", origin)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != http.StatusNoContent {
				t.Fatal("failed to switch the WebView language")
			}
			const description = "Existing description"
			const historicalComment = "Existing comment"
			card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Implementation", Description: description, ColumnID: flow.dev})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := core.AddComment(ctx, human, card.ID, historicalComment); err != nil {
				t.Fatal(err)
			}
			items := "- [x] " + tt.summary
			outcomes := []string{"plan or clarify", "ready or reported", "plan or clarify", "passed, failed or waiting"}
			calls := 0
			run.host = func(_ context.Context, _, dir, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
				// 关联检索的两次只读调用不属于规划、研发、审查，不计入阶段调用次数。
				if strings.Contains(prompt, "Outcome must be keywords.") {
					return dispatchReply{Outcome: "keywords", Summary: "[]"}, nil
				}
				if strings.Contains(prompt, "Outcome must be related.") {
					return dispatchReply{Outcome: "related", Summary: "[]"}, nil
				}
				if calls >= len(outcomes) {
					t.Fatal("unexpected extra host call")
				}
				for _, want := range []string{
					"Use " + tt.language + ` for all human-facing text in "summary" and "items"`,
					`Return only a final JSON object with string properties "outcome", "summary" and "items".`,
					"Outcome must be " + outcomes[calls] + ".",
					"Keep JSON field names, outcome values and Markdown checkbox syntax unchanged.",
				} {
					if !strings.Contains(prompt, want) {
						t.Errorf("host call %d lacks %q", calls+1, want)
					}
				}
				calls++
				if readOnly {
					return dispatchReply{Outcome: "plan", Summary: tt.plan}, nil
				}
				if calls == 2 {
					if err := os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("implementation "+tt.code), 0o600); err != nil {
						return dispatchReply{}, err
					}
					return dispatchReply{Outcome: "ready", Summary: tt.summary, Items: items}, nil
				}
				return dispatchReply{Outcome: "passed", Summary: tt.review}, nil
			}
			job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
			if err := run.handle(ctx, job); err != nil {
				t.Fatal(err)
			}
			got, err := core.GetCardDetail(ctx, card.ID)
			if err != nil {
				t.Fatal(err)
			}
			head, err := gitOutput(ctx, board.ProjectPath, "rev-parse", got.Branch)
			if err != nil || len(head) < 6 {
				t.Fatalf("no confirmed commit: %q (%v)", head, err)
			}
			if want := tt.summary + "\n\n" + fmt.Sprintf(tt.commitFormat, head[:6]); lastDesktopComment(got) != want {
				t.Errorf("handoff comment = %q, want %q", lastDesktopComment(got), want)
			}
			if want := description + "\n\n## " + tt.heading + "\n\n" + items; got.Description != want {
				t.Errorf("description = %q, want %q", got.Description, want)
			}
			if got.ColumnID != flow.review || len(got.Comments) < 2 || got.Comments[0].Body != historicalComment || got.Comments[1].Body != tt.plan {
				t.Fatal("development must hand off without altering previous comments or the host's plan")
			}

			job.review, job.engineer = true, *board.Engineers[2]
			if err := run.handle(ctx, job); err != nil {
				t.Fatal(err)
			}
			got, err = core.GetCardDetail(ctx, card.ID)
			if err != nil || got.ColumnID != flow.done || calls != len(outcomes) {
				t.Fatalf("unexpected review handoff: calls=%d err=%v", calls, err)
			}
			// 审查通过后桌面端先留审查证据（末尾带审批行），合并完成后再留一条合并说明，两条都跟随界面语言（合并提交列表由看板核心追加）。
			if len(got.Comments) < 2 || !strings.HasPrefix(got.Comments[len(got.Comments)-2].Body, tt.review+"\n\n"+approvalLinePrefix) ||
				!strings.HasPrefix(lastDesktopComment(got), desktopTexts[tt.code].dispatchMerged+"\n\n") || len(got.MergedCommits) != 1 {
				t.Fatalf("review evidence or merge note is not localized: %q", lastDesktopComment(got))
			}
		})
	}
}

// 新产生的派发异常和恢复留言随界面语言显示，但恢复指令和扩展字段仍使用原协议，
// 确保用户复制指令后仍能解除待核对状态。
func TestDesktopDispatchLocalizesFailureComments(t *testing.T) {
	core, human, board, flow := newDispatchTestBoard(t)
	for _, tt := range []struct {
		code, project, prior, formal, retry string
	}{
		{"en", "absolute local project path", "prior desktop dispatch ended", "could not confirm the formal execution result", "Confirm the prior host has stopped"},
		{"zh-CN", "绝对路径", "上一次桌面派发已结束", "未能确认正式执行结果", "确认之前的宿主已停止"},
		{"zh-TW", "絕對路徑", "上一次桌面派發已結束", "未能確認正式執行結果", "確認之前的宿主已停止"},
		{"ja", "絶対パス", "前回のデスクトップからの割り当て", "正式な実行結果を確認できませんでした", "以前のホストが停止したことを確認"},
	} {
		for _, failure := range []struct{ name, want string }{
			{"missing-project", tt.project}, {"prior-run", tt.prior}, {"formal-run", tt.formal},
		} {
			t.Run(tt.code+"/"+failure.name, func(t *testing.T) {
				ctx := t.Context()
				card, err := core.CreateCard(ctx, human, board.ID, kanban.CreateCardInput{Title: "Execution failure", ColumnID: flow.dev})
				if err != nil {
					t.Fatal(err)
				}
				job := dispatchJob{id: card.ID, engineer: *board.Engineers[0], host: "codex", flow: flow, project: board.ProjectPath}
				run := dispatchRun{core: core, sessions: t.TempDir(), text: func() desktopText { return desktopTexts[tt.code] }}
				run.host = func(_ context.Context, _, _, prompt string, readOnly bool, _ dispatchSession) (dispatchReply, error) {
					if strings.Contains(prompt, "Outcome must be keywords.") {
						return dispatchReply{Outcome: "keywords", Summary: "[]"}, nil
					}
					if strings.Contains(prompt, "Outcome must be related.") {
						return dispatchReply{Outcome: "related", Summary: "[]"}, nil
					}
					if failure.name != "formal-run" {
						t.Error("host should not start")
					}
					if readOnly {
						return dispatchReply{Outcome: "plan", Summary: "Prepared plan"}, nil
					}
					return dispatchReply{}, errUnconfirmedRun
				}
				if failure.name == "missing-project" {
					job.project = "relative-project"
				}
				if failure.name == "prior-run" {
					if _, err := core.AddCommentWithExt(ctx, job.actor(), card.ID, "", markerExt(job.startedMarker())); err != nil {
						t.Fatal(err)
					}
				}
				if err := run.handle(ctx, job); err == nil {
					t.Fatal("expected dispatch failure")
				}
				got, err := core.GetCardDetail(ctx, card.ID)
				if err != nil {
					t.Fatal(err)
				}
				if comment := lastDesktopComment(got); got.ClaimActive || got.ColumnID != flow.dev || !strings.Contains(comment, failure.want) {
					t.Errorf("unexpected failure comment or state: %q", comment)
				}
				if failure.name == "missing-project" {
					return
				}
				code := job.retryCode(lastDesktopMarker(got))
				command := "Slate desktop retry: " + code
				comment := lastDesktopComment(got)
				if code == "" || !strings.Contains(comment, tt.retry) || !strings.HasSuffix(comment, "\n\n`"+command+"`") || desktopDispatchRecovery(got, false) != dispatchAwaitingConfirmation {
					t.Errorf("unexpected recovery notice: %q", comment)
				}
				if _, err := core.AddComment(ctx, human, card.ID, command); err != nil {
					t.Fatal(err)
				}
				got, err = core.GetCardDetail(ctx, card.ID)
				if err != nil || desktopDispatchRecovery(got, false) != dispatchAvailable {
					t.Fatalf("copying the recovery command did not unlock dispatch: %v", err)
				}
			})
		}
	}
}
