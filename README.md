# Slate

**A kanban board for harness engineering and loop engineering.** You design the board: columns, prompts, and the engineers that set concurrency. Coding agents run the loop on it: claim a task, work in a git worktree, hand off for review, wait for the next one.

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

## Install

1. Open the [latest release](https://github.com/ixugo/slate-kanban/releases/latest) and download the package for your system:

| System | Package |
|---|---|
| macOS (Apple silicon) | `slate_<version>_darwin_arm64_install.tar.gz` |
| Linux (x86-64) | `slate_<version>_linux_amd64_install.tar.gz` |
| Linux (ARM64) | `slate_<version>_linux_arm64_install.tar.gz` |
| Windows (x86-64) | `slate_<version>_windows_amd64_install.zip` |

2. Extract it, then run Slate from the extracted folder. On Windows, unzip and run `slate.exe` inside the folder.

```sh
tar -xzf slate_*_install.tar.gz
cd slate_*/
./slate
```

If your release has no `_install` package, extract the package for your system into a folder of your choice and run Slate from there.

3. Open [http://127.0.0.1:7070](http://127.0.0.1:7070).

- If macOS refuses to open the file, run `xattr -d com.apple.quarantine slate` once.
- To run Slate in the background and start it at login (macOS and Linux), use `./slate -d`. Stop it and remove the service with `./slate -stop`.
- When a new release is out, Slate shows a notice in the browser. Choose **Update now**, then restart Slate.

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

## Data and access

- Data lives in `~/.slate-kanban/`. Stop Slate, then back up `data.db` and `resource/` together.
- Slate listens on `0.0.0.0:7070`. With no passcode set, anyone who can reach it can read and edit every board. Before sharing access, set `Server.HTTP.Passcode` and put Slate behind an HTTPS reverse proxy. See the [configuration example](configs/config.example.toml).

## License

See [LICENSE](LICENSE).
