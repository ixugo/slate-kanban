# Slate

**為 harness engineering 與 loop engineering 打造的看板。** 你來設計看板：欄、提示詞，以及決定並行數的工程師。程式開發 Agent 在上面跑迴圈：領取任務、在 git worktree 裡工作、交付審查、等待下一個。

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [日本語](README.ja.md)

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
- 有新 release 時，網頁會跳出提示。點選 **立即更新**，Slate 會自動重新啟動，頁面隨後自動重新整理。（以 Windows 服務執行時需要手動重新啟動服務。）

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

## MCP token 量測

量測使用隔離的 SQLite 工作區、固定的中英雙語樣本、MCP 2025-06-18 和 `o200k_base`（tiktoken 0.12.0）。樣本包括一個空看板和一個有四張任務卡的看板，其中一張長任務卡有 1,770 字元的描述、五則留言和兩項依賴。基準版本為 `a8a7a03`，複測版本為 `e5f8c54`。

| 工具探索 | 基準預設檢視 | 改版後預設檢視 | 基準管理檢視 | 改版後舊版管理檢視 | 改版後揭露管理工具 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 可見工具數 | 19 | 20 | 25 | 22 | 28 |
| 伺服器指引 | 817 | 153 | 241 | 40 | 153 |
| MCP `initialize` 回應 | 931 | 233 | 332 | 117 | 233 |
| MCP `tools/list` 回應 | 5,517 | 4,094 | 7,139 | 5,922 | 6,493 |
| Codex 0.170.0 送往 Responses 的 Slate 命名空間 | 6,031 | 3,898 | 6,943 | 5,566 | 用戶端保留預設的 3,898 |
| Codex 送往 Responses 的 `whoami` 結果物件 | 101 | 94 | 104 | 94 | — |

`enable_management` 在預設檢視新增了一個入口。移除三個舊工具後，各檢視合計的相異工具數由 30 減至 28。預設工具清單的 token 減少了 25.8%，Codex 送出的 Slate 命名空間減少了 35.4%。Codex 在命名空間中只包含一份伺服器指引。這些數值來自對擷取訊息的 tokenizer 計數，已登入宿主的測試未取得供應商用量。

| 七次呼叫的調度範例 | 請求 token | 回應 token |
| --- | ---: | ---: |
| 基準 | 277 | 13,737 |
| 改版後，沿用原參數 | 277 | 13,741 |
| 改版後，選用精簡看板讀取 | 285 | 8,044 |

沿用原參數時的微小差異來自樣本的時間戳記。精簡流程使用 `list_boards(summary=true)` 和 `get_board(include_watch_prompt=false)`。原有呼叫仍會回傳完整看板資料。改版前後，空等回應都是 93 token，留言喚醒等待的回應都是 261 token。事件監聽器可在模型回合之外等待。

[重現命令、各工具計數、呼叫明細與宿主驗證](docs/mcp-token-baseline.md#reproduce).

### 透過 MCP 管理 Slate

用標準設定連線一次 Slate MCP。需要變更看板設定、欄或自動化時，明確告知 Agent。Agent 會透過 `enable_management` 在目前的連線揭露管理工具。Codex 0.170.0 在同一對話的下一回合仍使用預設清單。工具清單固定的宿主可用 `include_tools: true` 取得目錄，再呼叫一次 `enable_management`，以 `tool` 指定管理工具名稱，並以 `arguments` 傳入參數。隔離的 Codex 測試透過此入口實際建立了欄。

既有工作連線與舊版管理連線仍可使用。移除的三個 MCP 工具可透過 REST 執行相同操作。用 `GET /api/v1/boards/{id}/events` 讀取活動，用 `DELETE /api/v1/cards/{id}` 刪除任務，用 `DELETE /api/v1/comments/{id}` 刪除留言。任務的規劃欄位可在舊版管理連線、Slate 網頁或 REST 中管理。新設定只需一條標準連線。

### 透過 MCP 搜尋任務

將搜尋詞與篩選條件傳給 `list_cards` 的 `query`，語法與網頁搜尋相同。關鍵字會比對標題、描述、任務編號與標籤。請將範例中的 `board_id` 換成要搜尋的看板編號。

```json
{"board_id":1,"query":"\"login timeout\" is:open","limit":10}
```

結果包含任務摘要、命中總數 `total` 與截斷標記 `truncated`。用 `get_card` 讀取命中任務的完整內容與留言。

更新搜尋說明後，預設 20 項工具的 `tools/list` 回應由 4,094 降至 4,092 token。[工具探索複測及重現方式](docs/mcp-token-baseline.md#discovery-retest-for-task-search)。

## 資料與存取

- 資料存放在 `~/.slate-kanban/`。備份前先停止 Slate，將 `data.db` 與 `resource/` 一起備份。
- Slate 監聽 `0.0.0.0:7070`。沒設口令時，能連上它的人都能讀寫所有看板。需要共享存取時，設定 `Server.HTTP.Passcode`，放在 HTTPS 反向代理後面。請參閱[設定範例](configs/config.example.toml)。

## 授權

請參閱 [LICENSE](LICENSE)。
