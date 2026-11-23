# Slate

**A kanban board for harness engineering and loop engineering.** You design the board: columns, prompts, and the engineers that set concurrency. Coding agents run the loop on it: claim a task, work in a git worktree, hand off for review, wait for the next one.

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

## Install

1. Open the [latest release](https://github.com/ixugo/slate-kanban/releases/latest) and download the package for your system:

| System | Package |
|---|---|
| macOS (Apple silicon) | `slate_<version>_darwin_arm64.tar.gz` |
| Linux (x86-64) | `slate_<version>_linux_amd64.tar.gz` |
| Linux (ARM64) | `slate_<version>_linux_arm64.tar.gz` |
| Windows (x86-64) | `slate_<version>_windows_amd64.zip` |

2. Extract it into a new folder, then run Slate from that folder. The package holds the program and the start/stop scripts (`start.sh` and `stop.sh`; `start.bat` and `stop.bat` on Windows). On Windows, unzip into a folder and run `slate.exe` there.

```sh
mkdir slate && tar -xzf slate_*.tar.gz -C slate
cd slate
./slate
```

3. Open [http://127.0.0.1:7070](http://127.0.0.1:7070).

- If macOS refuses to open the file, run `xattr -d com.apple.quarantine slate` once.
- To run Slate in the background and start it at login (macOS and Linux), use `./slate -d`. Stop it and remove the service with `./slate -stop`.
- When a new release is out, Slate shows a notice in the browser. Choose **Update now**: Slate restarts itself and the page refreshes automatically. (Running as a Windows service, restart the service by hand.)

## Get started

1. Select **New project** and keep the **Development** template: **Inbox → To do → In progress → Review → Done**. Inbox is yours; agents manage **To do** and **In progress**.
2. Open **Agents**, copy the one-line prompt into Claude Code, Cursor, or Codex. It installs the MCP server and the Slate skill.
3. Add tasks that say what to build and how to verify it. Move a task to **To do** when it is ready.
4. Keep an agent running: on **Agents**, find **Run long-term**, pick your project, and paste the prompt into an agent session started in the project directory (inside `screen` or `tmux`, so it survives closing the terminal). The agent loops: wait for updates, claim from **To do**, move to **In progress**, hand off to **Review**.
5. Check the handoff in **Review**. Move the task to **Done**, or comment and send it back to **In progress**.

## Shape the harness

- **Column prompts** (**Settings → Columns**) tell agents what they may do in each column.
- **Engineers** (**Settings → Engineers**) are concurrency slots: one engineer works one task at a time. Recruit full-stack, frontend, backend, UI, and test engineers.
- **Automation rules** (**Settings → Automation**) move tasks and set fields when something happens.

## MCP token measurement

The benchmark uses an isolated SQLite workspace, fixed Chinese and English fixtures, MCP 2025-06-18, and `o200k_base` (tiktoken 0.12.0). It uses an empty board and a board with four tasks, including a long task with a 1,770-character description, five comments, and two dependencies. The baseline is `a8a7a03`, and the retest is `e5f8c54`.

| Discovery | Baseline default | Updated default | Baseline management | Updated legacy management | Updated after disclosure |
| --- | ---: | ---: | ---: | ---: | ---: |
| Tools visible | 19 | 20 | 25 | 22 | 28 |
| Server instructions | 817 | 153 | 241 | 40 | 153 |
| MCP `initialize` reply | 931 | 233 | 332 | 117 | 233 |
| MCP `tools/list` reply | 5,517 | 4,094 | 7,139 | 5,922 | 6,493 |
| Codex 0.170.0 Slate namespace on the Responses wire | 6,031 | 3,898 | 6,943 | 5,566 | Client retains the default 3,898 |
| Codex `whoami` output object on the Responses wire | 101 | 94 | 104 | 94 | — |

`enable_management` adds one default entry. Removing three old tools reduces unique tool names across views from 30 to 28. Default tool-list tokens fell 25.8%, and the Slate namespace in Codex’s outbound JSON fell 35.4%. Codex serialized the instructions once in the namespace; no individual tool copied the full instructions. These are tokenizer counts of captured payloads. The signed-in host attempt produced no provider usage data.

| Seven-call dispatch sample | Request tokens | Reply tokens |
| --- | ---: | ---: |
| Baseline | 277 | 13,737 |
| Updated, existing arguments | 277 | 13,741 |
| Updated, opt-in compact board reads | 285 | 8,044 |

The small difference for existing arguments comes from fixture timestamps. The compact run uses `list_boards(summary=true)` and `get_board(include_watch_prompt=false)`; existing calls still work and include the full board data. Both versions measured 93 reply tokens for an empty wait and 261 for a comment waking a wait. The event listener can handle waits outside the model turn.

[Reproduction command, per-tool counts, tool calls, and host verification](docs/mcp-token-baseline.md#reproduce).

### Managing Slate through MCP

Connect once using the standard Slate MCP setup. Ask the agent explicitly when you want to change board settings, columns, or automation. The agent uses `enable_management` to disclose those tools in its current connection. Codex 0.170.0 retained the default list even on the next turn of the same thread. Hosts with a fixed list can request the catalog with `include_tools: true`, then call `enable_management` again with the management `tool` name and its `arguments`. This path created a real column in the isolated Codex host test.

Existing worker connections and legacy management connections remain usable. The three removed MCP tools have REST equivalents: activity uses `GET /api/v1/boards/{id}/events`, task deletion uses `DELETE /api/v1/cards/{id}`, and comment deletion uses `DELETE /api/v1/comments/{id}`. Card planning fields remain available in the legacy management connection and through the Slate web app or REST. Keep one standard client connection for new setups.

### Search tasks through MCP

Pass search terms and filters in the `query` of `list_cards`. It uses the same syntax as web search. Keywords match titles, descriptions, task keys, and tags. Replace `board_id` in this example with the ID of the board you want to search.

```json
{"board_id":1,"query":"\"login timeout\" is:open","limit":10}
```

Results contain card summaries, a `total` match count, and a `truncated` flag. Use `get_card` to read a matching task and its comments.

The search wording reduced the default `tools/list` reply from 4,094 to 4,092 tokens across the same 20 tools. [Discovery retest and reproduction](docs/mcp-token-baseline.md#discovery-retest-for-task-search).

## Data and access

- Data lives in `~/.slate-kanban/`. Stop Slate, then back up `data.db` and `resource/` together.
- Slate listens on `0.0.0.0:7070`. With no passcode set, anyone who can reach it can read and edit every board. Before sharing access, set `Server.HTTP.Passcode` and put Slate behind an HTTPS reverse proxy. See the [configuration example](configs/config.example.toml).

## License

See [LICENSE](LICENSE).
