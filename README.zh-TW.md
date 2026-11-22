# Slate

**為 harness engineering 與 loop engineering 打造的看板。** 你來設計看板：欄、提示詞，以及決定並行數的工程師。程式開發 Agent 在上面跑迴圈：領取任務、在 git worktree 裡工作、交付審查、等待下一個。

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

## 安裝

1. 開啟[最新 release](https://github.com/ixugo/slate-kanban/releases/latest)，下載對應系統的套件：

| 系統 | 套件 |
|---|---|
| macOS（Apple 晶片） | `slate_<version>_darwin_arm64.tar.gz` |
| Linux（x86-64） | `slate_<version>_linux_amd64.tar.gz` |
| Linux（ARM64） | `slate_<version>_linux_arm64.tar.gz` |
| Windows（x86-64） | `slate_<version>_windows_amd64.zip` |

2. 解壓縮到一個新目錄，進入該目錄執行。套件裡有程式和啟動、停止腳本（`start.sh`、`stop.sh`；Windows 是 `start.bat`、`stop.bat`）。Windows 解壓縮到一個目錄後執行其中的 `slate.exe`。

```sh
mkdir slate && tar -xzf slate_*.tar.gz -C slate
cd slate
./slate
```

3. 開啟 [http://127.0.0.1:7070](http://127.0.0.1:7070)。

- macOS 提示無法開啟時，執行一次 `xattr -d com.apple.quarantine slate`。
- 想讓 Slate 在背景執行並隨登入自動啟動（macOS 與 Linux），執行 `./slate -d`；停止並刪除服務用 `./slate -stop`。
- 有新 release 時，網頁會跳出提示。點選 **立即更新**，然後重新啟動 Slate。

## 開始使用

1. 選擇 **新增專案**，保留 **研發流程** 範本：**收件匣 → 待辦 → 進行中 → 審查 → 完成**。收件匣歸你；**待辦** 與 **進行中** 由 Agent 管理。
2. 開啟 **Agent** 頁面，把一句話提示複製給 Claude Code、Cursor 或 Codex，它會安裝 MCP 與 Slate skill。
3. 新增任務，寫清楚要做什麼、如何驗收。準備好後把任務放進 **待辦**。
4. 讓 Agent 長期執行：在 **Agent** 頁面找到 **長期掛著**，選好專案，把提示詞貼進在專案目錄中啟動的 Agent 工作階段（放在 `screen` 或 `tmux` 裡，關掉終端機也不會中斷）。Agent 會循環：等待更新、從 **待辦** 領取、移到 **進行中**、交付到 **審查**。
5. 在 **審查** 檢查交付內容。通過就移到 **完成**，不通過就留言並退回 **進行中**。

## 設計 harness

- **欄提示詞**（**設定 → 欄**）告訴 Agent 每一欄裡可以做什麼。
- **工程師**（**設定 → 工程師**）是並行名額：一位工程師同一時間只做一個任務。可以招募全端、前端、後端、UI 與測試工程師。
- **自動化規則**（**設定 → 自動化**）在事件發生時移動任務、設定欄位。

## 資料與存取

- 資料存放在 `~/.slate-kanban/`。備份前先停止 Slate，將 `data.db` 與 `resource/` 一起備份。
- Slate 監聽 `0.0.0.0:7070`。沒設口令時，能連上它的人都能讀寫所有看板。需要共享存取時，設定 `Server.HTTP.Passcode`，放在 HTTPS 反向代理後面。請參閱[設定範例](configs/config.example.toml)。

## 授權

請參閱 [LICENSE](LICENSE)。
