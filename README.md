# Slate

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md) · [한국어](README.ko.md) · [日本語](README.ja.md)

Slate is a local-first kanban board shared by people and AI agents. Create cards in the browser, move ready tasks into **In progress**, and let a coding agent pick them up over MCP. The agent hands its work off in **Review** so you can check it on the same board.

## Install with Docker

Install Git, Docker, and Docker Compose, then run the following commands.

```sh
git clone https://github.com/ixugo/slate-kanban.git
cd slate-kanban
docker compose up -d --build
```

Open [http://127.0.0.1:7070](http://127.0.0.1:7070). Compose builds Slate from this checkout and keeps the SQLite database, attachments, and logs in the `slate-data` volume. To update, rerun the Compose command from the same checkout after updating it. To stop Slate and keep your data, run `docker compose down`.

For a standalone executable or local development, see [Build and run from source (简体中文)](docs/development.zh-CN.md).

## Create your first project

1. Select **New project**, enter a name, and keep the **Development** template. It gives you **Inbox → To do → In progress → Review → Done**. Choose **Blank** to set up your own columns under **Settings → Columns**.
2. Select **New card** and describe the task and how to verify it. Add labels, a priority, a due date, or dependencies as needed.
3. Once your agent is connected, drag the card to **In progress** when it is ready to work on. When the work reaches **Review**, check the handoff and move the card to **Done** if it passes. If it needs changes, leave a comment and move it back to **In progress**.

## Connect an AI agent

Open **Agents** in Slate and copy the one-line prompt into Claude Code, Cursor, or Codex. For a local Slate instance, you can send the prompt below.

```text
Read http://127.0.0.1:7070/install.md and follow the steps to set up Slate.
```

You can also use the **Agents** page to copy the MCP configuration and skill installation commands for your client. Give each agent its own `name` in the MCP URL. Use the worker configuration for daily work and the optional manager configuration to edit columns or automation rules.

After installing both MCP and the Slate skill, open a new agent session and ask it to call `whoami`. Ask it to work on your project. When it asks which repository belongs to the project, confirm the local path so it can work in the right Git worktree.

## Use the board

- Change the interface language from the top bar. Slate supports English, Simplified Chinese, Traditional Chinese, Korean, and Japanese.
- Switch between the board and the sortable list. Use search or the filter panel to find cards. For example, `is:overdue -is:done` finds unfinished cards past their due date.
- Use **Settings → Automation** to add rules. The Development template records when cards enter **In progress** and **Done**, and clears the completion time when a card leaves **Done**.
- Use **Settings → Export** to download your project as JSON or Markdown.
- On localhost or HTTPS, use your browser’s install option to open Slate in its own window. Offline, you can view the last cached board snapshot.

## Data and access

- Native runs store data under `~/.slate-kanban/`. Back up `data.db` and the `resource/` directory together while Slate is stopped. With Docker, keep the `slate-data` volume when updating or recreating the container.
- Slate listens on all IPv4 interfaces at `0.0.0.0:7070` by default. With the passcode unset, anyone who can reach it can read and edit every board. Before sharing access, set `Server.HTTP.Passcode`, serve Slate behind an HTTPS reverse proxy, and add the public hostname to `Server.HTTP.AllowedHosts`. See the configuration examples for [native installs](configs/config.example.toml) and [Docker](configs/config.docker.toml).

## License

See [LICENSE](LICENSE).
