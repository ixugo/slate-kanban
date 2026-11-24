package desktop

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"slate-kanban/internal/app"
	"slate-kanban/internal/core/kanban"
)

type desktopText struct {
	window, minimise                                                               string
	navigate, show, boards, agents, quit                                           string
	empty, unavailable, startup, inUse, confirm                                    string
	listenUnavailable                                                              string
	edit, undo, redo, cut, copy, paste, selectAll                                  string
	about, settings, hide, hideOthers, showAll                                     string
	dispatchNode, dispatchContext, dispatchPlan, dispatchSession, dispatchWorktree string

	dispatchOutputLanguage, dispatchProjectPath                 string
	dispatchPriorUnconfirmed, dispatchUnconfirmed               string
	dispatchRetryInstruction, dispatchCommit, dispatchTaskItems string

	dispatchMerged, dispatchMergeBlocked, dispatchMergeConflict string
}

var desktopTexts = map[string]desktopText{
	"en": {
		window: "Window", minimise: "Minimize",
		navigate: "Navigate", show: "Show Slate", boards: "Boards", agents: "Agent Setup", quit: "Quit Slate",
		empty: "No recent cards", unavailable: "Recent cards unavailable", startup: "Slate could not start. Check its configuration and try again.",
		inUse: "Slate is already using this configuration. Quit the running server or desktop app, then open Slate again.", confirm: "OK",
		listenUnavailable: "Slate could not listen on %s. Check the listening address and port in config.toml. If the port is in use, quit the program using it before reopening Slate.",
		edit:              "Edit", undo: "Undo", redo: "Redo", cut: "Cut", copy: "Copy", paste: "Paste", selectAll: "Select All",
		about: "About Slate", settings: "Settings…", hide: "Hide Slate", hideOthers: "Hide Others", showAll: "Show All",
		dispatchNode:     "The desktop host launcher could not find Node.js. Install Node.js or make it available to Slate, then retry dispatch.",
		dispatchContext:  "Slate could not retrieve reliable related task context. Check the database and the selected host CLI, then retry dispatch.",
		dispatchPlan:     "The desktop host did not produce a usable task plan. Check that the selected host CLI starts and that its account is configured, then retry dispatch.",
		dispatchSession:  "Slate could not prepare the local task session. Check write access to the Slate configuration directory, then retry dispatch.",
		dispatchWorktree: "Slate could not prepare the task worktree. Check the board's Git project path, task branch and worktree changes before retrying dispatch.",

		dispatchOutputLanguage:   "English",
		dispatchProjectPath:      "Desktop dispatch needs this board's absolute local project path.",
		dispatchPriorUnconfirmed: "A prior desktop dispatch ended without a confirmed result.",
		dispatchUnconfirmed:      "Desktop dispatch could not confirm the formal execution result.",
		dispatchRetryInstruction: "Confirm the prior host has stopped, then inspect the task worktree and results. To authorize another dispatch, post this exact text as a new human comment:",
		dispatchCommit:           "Commit: `%s`.", dispatchTaskItems: "Task items",

		dispatchMerged:        "Slate desktop integrated the reviewed commits into the project's current branch.",
		dispatchMergeBlocked:  "Slate desktop could not integrate the approved branch:\n\n```\n%s\n```\n\nThe review result is kept and the card stays in Review. Fix the cause, then add a comment on the card to try the integration again.",
		dispatchMergeConflict: "The approved branch conflicts with the project's current branch `%s`. The card returns to In Progress. Rebase the task branch onto commit `%s`, resolve the conflicts, run the checks and commit. The result is reviewed again before it is integrated.",
	},
	"zh-CN": {
		window: "窗口", minimise: "最小化",
		navigate: "导航", show: "显示 Slate", boards: "看板", agents: "接入 Agent", quit: "退出 Slate",
		empty: "暂无最近任务", unavailable: "无法读取最近任务", startup: "Slate 无法启动。请检查配置后重试。",
		inUse: "已有 Slate 进程在使用此配置。请退出运行中的服务端或桌面应用，再打开 Slate。", confirm: "确定",
		listenUnavailable: "Slate 无法监听 %s。请检查 config.toml 中的监听地址和端口。如果端口已被占用，请退出占用该端口的程序，再打开 Slate。",
		edit:              "编辑", undo: "撤销", redo: "重做", cut: "剪切", copy: "复制", paste: "粘贴", selectAll: "全选",
		about: "关于 Slate", settings: "设置…", hide: "隐藏 Slate", hideOthers: "隐藏其他", showAll: "显示全部",
		dispatchNode:     "桌面宿主启动时找不到 Node.js。请安装 Node.js 或让 Slate 能找到它，再重试派发。",
		dispatchContext:  "Slate 无法检索到可靠的关联任务上下文。请检查数据库和所选宿主命令行，再重试派发。",
		dispatchPlan:     "桌面宿主未生成可用的任务方案。请检查所选宿主命令行能否启动、账号是否已配置，再重试派发。",
		dispatchSession:  "Slate 无法准备本地任务会话。请检查 Slate 配置目录的写入权限，再重试派发。",
		dispatchWorktree: "Slate 无法准备任务工作树。请检查看板的 Git 项目路径、任务分支和工作树改动，再重试派发。",

		dispatchOutputLanguage:   "Simplified Chinese (简体中文)",
		dispatchProjectPath:      "桌面派发需要此看板配置本地项目的绝对路径。",
		dispatchPriorUnconfirmed: "上一次桌面派发已结束，但未确认执行结果。",
		dispatchUnconfirmed:      "桌面派发未能确认正式执行结果。",
		dispatchRetryInstruction: "请确认之前的宿主已停止，然后检查任务工作树和执行结果。要授权重新派发，请复制下方完整文字，作为一条新的人工评论发送：",
		dispatchCommit:           "提交：`%s`。", dispatchTaskItems: "任务项",

		dispatchMerged:        "Slate 桌面端已把审查通过的提交合入项目当前分支。",
		dispatchMergeBlocked:  "Slate 桌面端无法合并已审查通过的分支：\n\n```\n%s\n```\n\n审查结论已保留，任务停在审查列。处理好原因后，在卡片下留一条评论即可重试合并。",
		dispatchMergeConflict: "已审查通过的分支与项目当前分支 `%s` 有冲突，任务退回「进行中」。请把任务分支变基到提交 `%s`，解决冲突、运行检查并提交。结果会重新审查，再合并。",
	},
	"zh-TW": {
		window: "視窗", minimise: "縮到最小",
		navigate: "導覽", show: "顯示 Slate", boards: "看板", agents: "連接 Agent", quit: "結束 Slate",
		empty: "目前沒有最近任務", unavailable: "無法讀取最近任務", startup: "Slate 無法啟動。請檢查設定後再試一次。",
		inUse: "已有 Slate 程序正在使用此設定。請結束執行中的伺服器或桌面應用程式，再開啟 Slate。", confirm: "確定",
		listenUnavailable: "Slate 無法監聽 %s。請檢查 config.toml 中的監聽位址和連接埠。如果連接埠已被占用，請結束占用該連接埠的程式，再開啟 Slate。",
		edit:              "編輯", undo: "復原", redo: "重做", cut: "剪下", copy: "複製", paste: "貼上", selectAll: "全選",
		about: "關於 Slate", settings: "設定…", hide: "隱藏 Slate", hideOthers: "隱藏其他", showAll: "顯示全部",
		dispatchNode:     "桌面宿主啟動時找不到 Node.js。請安裝 Node.js 或讓 Slate 能找到它，再重試派發。",
		dispatchContext:  "Slate 無法檢索到可靠的關聯任務上下文。請檢查資料庫和所選宿主命令列，再重試派發。",
		dispatchPlan:     "桌面宿主未產生可用的任務方案。請檢查所選宿主命令列能否啟動、帳號是否已設定，再重試派發。",
		dispatchSession:  "Slate 無法準備本機任務工作階段。請檢查 Slate 設定目錄的寫入權限，再重試派發。",
		dispatchWorktree: "Slate 無法準備任務工作樹。請檢查看板的 Git 專案路徑、任務分支和工作樹變更，再重試派發。",

		dispatchOutputLanguage:   "Traditional Chinese (繁體中文)",
		dispatchProjectPath:      "桌面派發需要此看板設定本機專案的絕對路徑。",
		dispatchPriorUnconfirmed: "上一次桌面派發已結束，但未確認執行結果。",
		dispatchUnconfirmed:      "桌面派發未能確認正式執行結果。",
		dispatchRetryInstruction: "請確認之前的宿主已停止，然後檢查任務工作樹和執行結果。若要授權重新派發，請複製下方完整文字，作為一則新的人工留言送出：",
		dispatchCommit:           "提交：`%s`。", dispatchTaskItems: "任務項",

		dispatchMerged:        "Slate 桌面端已將審查通過的提交合入專案目前分支。",
		dispatchMergeBlocked:  "Slate 桌面端無法合併已審查通過的分支：\n\n```\n%s\n```\n\n審查結論已保留，任務停在審查列。處理好原因後，在卡片下留一則評論即可重試合併。",
		dispatchMergeConflict: "已審查通過的分支與專案目前分支 `%s` 有衝突，任務退回「進行中」。請把任務分支變基到提交 `%s`，解決衝突、執行檢查並提交。結果會重新審查，再合併。",
	},
	"ja": {
		window: "ウインドウ", minimise: "しまう",
		navigate: "移動", show: "Slate を表示", boards: "ボード", agents: "エージェント設定", quit: "Slate を終了",
		empty: "最近のカードはありません", unavailable: "最近のカードを読み込めません", startup: "Slate を起動できませんでした。設定を確認してからやり直してください。",
		inUse: "この設定は別の Slate で使用中です。実行中のサーバーまたはデスクトップアプリを終了し、Slate を開き直してください。", confirm: "OK",
		listenUnavailable: "Slate は %s で待ち受けできませんでした。config.toml の待ち受けアドレスとポートを確認してください。ポートが使用中の場合は、そのアプリを終了してから Slate を開き直してください。",
		edit:              "編集", undo: "取り消す", redo: "やり直す", cut: "切り取り", copy: "コピー", paste: "貼り付け", selectAll: "すべてを選択",
		about: "Slate について", settings: "設定…", hide: "Slate を隠す", hideOthers: "ほかを隠す", showAll: "すべてを表示",
		dispatchNode:     "デスクトップホストの起動時に Node.js が見つかりません。Node.js をインストールするか、Slate が見つけられるようにしてから再試行してください。",
		dispatchContext:  "Slate は信頼できる関連タスクのコンテキストを取得できませんでした。データベースと選択したホストの CLI を確認してから再試行してください。",
		dispatchPlan:     "デスクトップホストが有効なタスク計画を生成できませんでした。選択したホストの CLI が起動し、アカウントが設定されていることを確認してから再試行してください。",
		dispatchSession:  "Slate はローカルのタスクセッションを準備できませんでした。設定ディレクトリの書き込み権限を確認してから再試行してください。",
		dispatchWorktree: "Slate はタスクの作業ツリーを準備できませんでした。ボードの Git プロジェクトパス、タスクブランチ、作業ツリーの変更を確認してから再試行してください。",

		dispatchOutputLanguage:   "Japanese (日本語)",
		dispatchProjectPath:      "デスクトップからタスクを割り当てるには、ボードのローカルプロジェクトの絶対パスが必要です。",
		dispatchPriorUnconfirmed: "前回のデスクトップからの割り当ては終了しましたが、実行結果を確認できませんでした。",
		dispatchUnconfirmed:      "デスクトップで正式な実行結果を確認できませんでした。",
		dispatchRetryInstruction: "以前のホストが停止したことを確認してから、タスクの作業ツリーと実行結果を確認してください。再割り当てを許可するには、次の文字列をそのまま新しい手動コメントとして投稿してください：",
		dispatchCommit:           "コミット：`%s`。", dispatchTaskItems: "タスク項目",

		dispatchMerged:        "Slate デスクトップが、レビュー済みのコミットをプロジェクトの現在のブランチに統合しました。",
		dispatchMergeBlocked:  "Slate デスクトップは承認済みのブランチを統合できませんでした：\n\n```\n%s\n```\n\nレビュー結果は保持され、カードはレビューに残ります。原因を解消してから、カードにコメントを追加すると統合を再試行します。",
		dispatchMergeConflict: "承認済みのブランチがプロジェクトの現在のブランチ `%s` と競合しています。カードは進行中に戻ります。タスクブランチをコミット `%s` にリベースし、競合を解消してチェックを実行し、コミットしてください。結果は再度レビューされてから統合されます。",
	},
}

func (t desktopText) startupMessage(err error) string {
	if errors.Is(err, app.ErrConfigInUse) {
		return t.inUse
	}
	if listenErr, ok := errors.AsType[*net.OpError](err); ok && listenErr.Op == "listen" && listenErr.Addr != nil {
		return fmt.Sprintf(t.listenUnavailable, listenErr.Addr)
	}
	return t.startup
}

func preferredLanguage(candidates []string) string {
	for _, tag := range candidates {
		tag, _, _ = strings.Cut(tag, ".")
		tag = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
		base, _, _ := strings.Cut(tag, "-")
		switch base {
		case "en", "zh", "ja":
			return kanban.NormalizeLocale(tag)
		}
	}
	return "en"
}

// desktopLanguage 跟随 WebView 已有的界面语言，并保存原生菜单的最后一次选择，
// 这样下次启动失败时，原生提示也能用上次的语言。
type desktopLanguage struct {
	file    string
	path    string
	current atomic.Pointer[string]
	changed chan struct{}
}

func newDesktopLanguage(dir string) *desktopLanguage {
	lang := &desktopLanguage{path: "/.slate-desktop/" + rand.Text(), changed: make(chan struct{}, 1)}
	if dir != "" {
		lang.file = filepath.Join(dir, "desktop-language")
	}
	saved, _ := os.ReadFile(lang.file)
	code := strings.TrimSpace(string(saved))
	if _, ok := desktopTexts[code]; !ok {
		code = preferredLanguage(systemLanguages())
	}
	lang.current.Store(new(code))
	return lang
}

func (l *desktopLanguage) text() desktopText {
	return desktopTexts[*l.current.Load()]
}

// script runs only in the native window on the configured Slate origin.
// It observes the same language that React has already chosen and applied to HTML.
func (l *desktopLanguage) script(origin string) string {
	return fmt.Sprintf(`(() => {
		if (location.origin !== %q) return;
		let last;
		let requests = Promise.resolve();
		const send = () => {
			const language = document.documentElement.lang;
			if (!language || language === last) return;
			last = language;
			requests = requests.then(() => fetch(%q, {
				method: 'POST', body: language, keepalive: true
			})).catch(() => {});
		};
		new MutationObserver(send).observe(document.documentElement, {attributeFilter: ['lang']});
		send();
	})()`, origin, l.path)
}

// wrap installs a desktop-only endpoint. The random path is sent only to the
// native WebView, and same-origin requests alone may update the locale.
func (l *desktopLanguage) wrap(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != l.path {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Origin") != origin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 12))
		code := string(raw)
		if _, ok := desktopTexts[code]; err != nil || !ok {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if code != *l.current.Swap(new(code)) {
			if err := os.WriteFile(l.file, raw, 0o600); err != nil {
				slog.Warn("保存桌面语言失败", "err", err)
			}
			select {
			case l.changed <- struct{}{}:
			default:
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
