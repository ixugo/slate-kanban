# Slate

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

Slate 是一個以本機使用為主、由人和 AI Agent 共用的看板。在網頁上建立卡片，把準備就緒的任務拖到 **進行中**，程式開發 Agent 就能透過 MCP 領取。工作完成後，Agent 會把卡片交到 **審查**，方便你在同一個看板上驗收。

## 使用 Docker 安裝

先安裝 Git、Docker 和 Docker Compose，再執行以下指令。

```sh
git clone https://github.com/ixugo/slate-kanban.git
cd slate-kanban
docker compose up -d --build
```

開啟 [http://127.0.0.1:7070](http://127.0.0.1:7070)。Compose 會以目前的原始碼建置 Slate，並將 SQLite 資料庫、附件及記錄檔存放在 `slate-data` 儲存卷。升級時，更新同一個目錄中的原始碼，再執行相同的 Compose 指令。若要停止 Slate 並保留資料，請執行 `docker compose down`。

若要建置單一執行檔或在本機開發，請參閱 [原始碼建置與開發啟動（简体中文）](docs/development.zh-CN.md)。

## 建立第一個專案

1. 選擇 **新增專案**，輸入名稱，保留預設的 **研發流程** 範本。看板會建立 **收件匣 → 待辦 → 進行中 → 審查 → 完成**。也可以選擇 **空白**，到 **設定 → 欄** 自行新增。
2. 選擇 **新增卡片**，寫清楚任務與驗收條件，並視需要新增標籤、優先順序、截止日期或相依項目。
3. 連接 Agent 後，將準備就緒的卡片拖到 **進行中** 交給 Agent。卡片進入 **審查** 後，檢查交接結果。通過就移到 **完成**，需要修改就留下評論並移回 **進行中**。

## 連接 AI Agent

開啟 Slate 的 **Agent** 頁面，將一句話提示複製到 Claude Code、Cursor 或 Codex。在本機執行時，可以直接傳送下面這句話。

```text
請閱讀 http://127.0.0.1:7070/install.md ，依照說明完成 Slate 設定。
```

也可以從 **Agent** 頁面複製對應用戶端的 MCP 設定和 skill 安裝指令。讓每個 Agent 使用獨立的 MCP `name`。平時領取卡片使用工作檢視，需要修改欄或自動化規則時再設定管理檢視。

裝好 MCP 與 Slate skill 後，開啟新的 Agent 工作階段，請它呼叫 `whoami` 確認連線，再請它處理你的專案。當它詢問對應的 Git 儲存庫時，確認本機路徑，讓它在對應的 Git worktree 中工作。

## 使用看板

- 從頂端工具列切換介面語言，支援繁體中文、簡體中文、英文、韓文及日文。
- 在看板及可排序的清單之間切換，透過搜尋或篩選面板找卡片。例如，`is:overdue -is:done` 會找出已逾期且未完成的卡片。
- 到 **設定 → 自動化** 新增規則。研發流程範本會記錄卡片進入 **進行中** 和 **完成** 的時間。卡片離開 **完成** 時會清除完成時間。
- 到 **設定 → 匯出**，將專案下載為 JSON 或 Markdown。
- 透過 localhost 或 HTTPS 開啟 Slate 時，可使用瀏覽器的安裝功能將它放在獨立視窗中。離線時可以查看上次快取的看板快照。

## 資料與存取

- 直接執行 Slate 時，資料預設放在 `~/.slate-kanban/`。先停止 Slate，再一起備份 `data.db` 和 `resource/` 目錄。使用 Docker 時，升級或重建容器請保留 `slate-data` 儲存卷。
- Slate 預設監聽 `0.0.0.0:7070`，可透過本機與區域網路位址存取。未設定存取口令時，能連上 Slate 的人都可以讀寫所有看板。若要開放給其他人，請設定 `Server.HTTP.Passcode`，使用 HTTPS 反向代理，並在 `Server.HTTP.AllowedHosts` 填入對外網域名稱。請參閱 [本機設定範例](configs/config.example.toml) 和 [Docker 設定範例](configs/config.docker.toml)。

## 授權

請參閱 [LICENSE](LICENSE)。
