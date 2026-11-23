#!/usr/bin/env python3
"""Watch a Slate board with Python 3.9+ and launch a local agent only to dispatch work.

The listener and dispatcher use the same Slate identity. Keep one listener for that
identity, with its state directory on local disk. See --help for recovery.
"""
import argparse
from contextlib import contextmanager
import errno
import hashlib
from http.client import HTTPException
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener
from uuid import uuid4

PROTOCOL_VERSION = "2025-06-18"
PASSCODE_ENV = "SLATE_PASSCODE"
DONE = "SLATE_DISPATCH_DONE"
RECEIPT_OUTCOMES = ("processed", "waiting_external", "unconfirmed")
RECOVERY = ("Use the same launch arguments with --status to inspect the saved outcome. "
            "Make sure the previous dispatcher has stopped and verify its work and Slate card state. "
            "Use --resolve done if handled, or --resolve retry only if safe to run again. "
            "Then restart the listener with its original arguments.")
SUPERVISOR_INTERVAL = 300
SUPERVISOR_MAX_STARTS = 3
NEEDS_ATTENTION = 2
HOST_EXECUTABLES = {
    "codex": ("codex",),
    "cursor": ("cursor-agent",),
    "claude": ("claude",),
    "opencode": ("opencode",),
    "pi": ("pi",),
    "on-my-pi": ("omp", "on-my-pi"),
}
HOST_ALIASES = {"claude-code": "claude", "cursor-agent": "cursor", "omp": "on-my-pi"}


class MCPError(RuntimeError):
    pass


class PendingDispatch(RuntimeError):
    pass


class DispatchFailure(PendingDispatch):
    """A diagnostic generated locally, safe to persist without host output."""


class ListenerBusy(PendingDispatch):
    pass


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class SlateMCP:
    def __init__(self, url):
        self.url = url
        self.opener = build_opener(NoRedirect())
        self.session = None
        self.protocol = PROTOCOL_VERSION
        self.request_id = 0

    def send(self, method, params, timeout=10, *, notification=False):
        payload = {"jsonrpc": "2.0", "method": method, "params": params}
        if not notification:
            self.request_id += 1
            payload["id"] = self.request_id
        headers = {
            "Accept": "application/json, text/event-stream",
            "Content-Type": "application/json",
            "MCP-Protocol-Version": self.protocol,
        }
        if self.session:
            headers["Mcp-Session-Id"] = self.session
        passcode = os.environ.get(PASSCODE_ENV)
        if passcode:
            headers["Authorization"] = "Bearer " + passcode
        request = Request(self.url, json.dumps(payload).encode(), headers)
        with self.opener.open(request, timeout=timeout) as response:
            if method == "initialize":
                self.session = response.headers.get("Mcp-Session-Id")
            if notification:
                return None
            if response.headers.get_content_type() != "application/json":
                raise MCPError("Slate MCP did not return a JSON response")
            reply = json.load(response)
        if reply.get("id") != self.request_id or "error" in reply or "result" not in reply:
            raise MCPError("Invalid Slate MCP response to " + method)
        return reply["result"]

    def connect(self):
        self.session = None
        self.protocol = PROTOCOL_VERSION
        result = self.send("initialize", {
            "protocolVersion": PROTOCOL_VERSION,
            "capabilities": {},
            "clientInfo": {"name": "slate-watch", "version": "1"},
        })
        if not self.session:
            raise MCPError("Slate did not return a session ID")
        self.protocol = result["protocolVersion"]
        self.send("notifications/initialized", {}, notification=True)

    def tool(self, name, arguments, timeout):
        response = self.send("tools/call", {"name": name, "arguments": arguments}, timeout)
        if response.get("isError"):
            raise MCPError(name + " failed: " + response["content"][0]["text"])
        return response["structuredContent"]


class WatchState:
    def __init__(self, path):
        self.db = sqlite3.connect(path)
        with self.db:
            self.db.execute("""
                CREATE TABLE IF NOT EXISTS watch_state (
                    id INTEGER PRIMARY KEY CHECK (id = 1),
                    bootstrapped INTEGER NOT NULL DEFAULT 0,
                    cursor INTEGER,
                    pending_cursor INTEGER,
                    pending_host TEXT,
                    pending_error TEXT
                )
            """)
            columns = {row[1] for row in self.db.execute("PRAGMA table_info(watch_state)")}
            if "pending_host" not in columns:
                self.db.execute("ALTER TABLE watch_state ADD COLUMN pending_host TEXT")
            if "pending_error" not in columns:
                self.db.execute("ALTER TABLE watch_state ADD COLUMN pending_error TEXT")
            self.db.execute("INSERT OR IGNORE INTO watch_state (id) VALUES (1)")

    def close(self):
        self.db.close()

    def status(self):
        return self.db.execute(
            "SELECT bootstrapped, cursor, pending_cursor FROM watch_state WHERE id = 1"
        ).fetchone()

    @property
    def cursor(self):
        return self.status()[1]

    def dispatch_status(self):
        return read_watch_status(self.db)[3:]

    def pending(self, cursor, host=None):
        with self.db:
            self.db.execute(
                "UPDATE watch_state SET pending_cursor = ?, pending_host = ?, pending_error = NULL WHERE id = 1",
                (cursor, host),
            )

    def dispatch_failed(self, reason):
        with self.db:
            self.db.execute("UPDATE watch_state SET pending_error = ? WHERE id = 1", (reason,))

    def processed(self, cursor):
        with self.db:
            self.db.execute(
                """UPDATE watch_state SET bootstrapped = 1, cursor = ?, pending_cursor = NULL,
                    pending_host = NULL, pending_error = NULL WHERE id = 1""",
                (cursor,),
            )

    def resolve(self, decision):
        _, _, pending = self.status()
        if pending is None:
            raise PendingDispatch("There is no pending dispatch to resolve")
        if decision == "done":
            self.processed(pending)
        else:
            with self.db:
                self.db.execute("""UPDATE watch_state SET pending_cursor = NULL,
                    pending_host = NULL, pending_error = NULL WHERE id = 1""")


def read_watch_status(db):
    # Read old as well as migrated databases without changing them, including while the listener runs.
    columns = {row[1] for row in db.execute("PRAGMA table_info(watch_state)")}
    if not columns:
        return 0, None, None, None, None
    host_sql = "pending_host" if "pending_host" in columns else "NULL"
    error_sql = "pending_error" if "pending_error" in columns else "NULL"
    row = db.execute(f"SELECT bootstrapped, cursor, pending_cursor, {host_sql}, {error_sql} "
                     "FROM watch_state WHERE id = 1").fetchone()
    if not row:
        return 0, None, None, None, None
    bootstrapped, cursor, pending, host, reason = row
    if pending is None:
        return bootstrapped, cursor, pending, None, None
    return bootstrapped, cursor, pending, host, reason


def saved_status(args, path):
    bootstrapped, cursor, pending, host, reason = 0, None, None, None, None
    if path.exists():
        db = sqlite3.connect(path.absolute().as_uri() + "?mode=ro", uri=True)
        try:
            bootstrapped, cursor, pending, host, reason = read_watch_status(db)
        finally:
            db.close()
    return {
        "board": args.board, "name": args.name, "host": host,
        "bootstrapped": bool(bootstrapped), "cursor": cursor, "pending_cursor": pending,
        "failure": reason, "recovery": RECOVERY if pending is not None else None,
    }


def pending_message(state, pending):
    host, reason = state.dispatch_status()
    reason = reason or "outcome_not_recorded"
    return f"Dispatch needs review (host={host or 'unknown'}, cursor={pending}, failure={reason}). {RECOVERY}"


@contextmanager
def listener_lock(path):
    with path.open("a+b") as file:
        if file.tell() == 0:
            file.write(b"\0")
            file.flush()
        take_listener_lock(file)
        yield


def take_listener_lock(file):
    file.seek(0)
    try:
        if os.name == "nt":
            import msvcrt
            msvcrt.locking(file.fileno(), msvcrt.LK_NBLCK, 1)
        else:
            import fcntl
            fcntl.flock(file, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError as exc:
        if exc.errno not in (errno.EAGAIN, errno.EACCES):
            raise
        raise ListenerBusy("Another listener owns this state file") from exc


def identity_in_use(path):
    try:
        with path.with_suffix(".lock").open("r+b") as file:
            take_listener_lock(file)
    except ListenerBusy:
        return True
    except FileNotFoundError:
        pass
    return False


def handle_batch(state, dispatch, updates, *, host=None):
    bootstrapped, _, pending = state.status()
    if pending is not None:
        raise PendingDispatch(pending_message(state, pending))
    cursor = updates["cursor"]
    events = updates["events"]
    reconcile = not bootstrapped or updates.get("first") or updates.get("truncated")
    if events or reconcile:
        state.pending(cursor, host)
        try:
            dispatch(events, reconcile)
        except DispatchFailure as error:
            state.dispatch_failed(str(error))
            raise PendingDispatch(pending_message(state, cursor)) from error
    state.processed(cursor)


def watch_once(state, client, dispatch, board, timeout, *, more=False, host=None):
    bootstrapped, cursor, pending = state.status()
    if pending is not None:
        raise PendingDispatch(pending_message(state, pending))
    arguments = {"board_id": board}
    if cursor is not None:
        arguments["since"] = cursor
    if not bootstrapped or more:
        updates = client.tool("pull_updates", arguments, 10)
    else:
        arguments["timeout_seconds"] = timeout
        updates = client.tool("wait_for_updates", arguments, timeout + 10)
    handle_batch(state, dispatch, updates, host=host)
    return updates.get("has_more", False)


def local_executable(candidates):
    for candidate in candidates:
        executable = shutil.which(candidate)
        # Windows can implicitly invoke a shell for batch files even with shell=False.
        if executable and not (os.name == "nt" and Path(executable).suffix.lower() in (".bat", ".cmd")):
            return os.path.abspath(executable)
    return None


def resolve_dispatcher(args):
    if args.command_json:
        executable = local_executable(args.command_json[:1])
        if not executable:
            raise PendingDispatch("Custom executable not found or requires a shell")
        return "custom", [executable, *args.command_json[1:]]
    if args.codex:
        executable = local_executable((args.codex,))
        if not executable:
            raise PendingDispatch("Codex executable not found; check --codex")
        return "codex", [executable]

    host = args.host
    if host == "auto":
        # The standard Slate identity preserves existing launch commands.
        host = HOST_ALIASES.get(args.name, args.name)
        if host not in HOST_EXECUTABLES:
            available = {
                host: executable for host, names in HOST_EXECUTABLES.items()
                if (executable := local_executable(names))
            }
            if len(available) == 1:
                host, executable = next(iter(available.items()))
                return host, [executable]
            choices = ", ".join(available) or "none"
            raise PendingDispatch(f"Available hosts: {choices}. Set --host or --command-json.")

    names = HOST_EXECUTABLES[host]
    if host == "cursor":
        # The generic "agent" binary is only eligible once Cursor was selected.
        names = (*names, "agent")
    executable = local_executable(names)
    if not executable:
        raise PendingDispatch(f"{host} executable not found; install its CLI or use --command-json")
    return host, [executable]


def command_argv(value):
    try:
        command = json.loads(value)
    except json.JSONDecodeError as exc:
        raise argparse.ArgumentTypeError("--command-json must be a JSON array of arguments") from exc
    if not isinstance(command, list) or not command or not command[0] or any(
        not isinstance(arg, str) or "\0" in arg for arg in command
    ):
        raise argparse.ArgumentTypeError("--command-json must be a nonempty JSON array of strings")
    return command


def dispatch_prompt(args, events, reconcile, dispatch_id=None):
    changes = [{field: event.get(field) for field in ("id", "type", "card_id")} for event in events]
    if dispatch_id is None:
        receipt = f"本轮已处理或无事可做时，最终回复严格使用单独一行 {DONE}。\n" + (
            "执行失败、无法核实权限或结果、留有未跟进子 agent 时，报告实际情况，且不要输出该成功标记。"
        )
    else:
        receipt = f"""最终回复只填结构化回执，逐字复制本轮 dispatch_id: {dispatch_id}。
outcome 选择 processed（本轮已处理或无事可做）、waiting_external（本轮已处理，卡片等待外部条件）或 unconfirmed（执行失败、无法核实权限或结果、还有子 agent 需跟进）。
失败时按看板规则记录实际情况，最终仍返回 unconfirmed。"""
    return f"""你是 Slate 看板 {args.board} 的事件调度者，身份为 {args.name}。
只执行事件触发的一轮调度。使用 slate MCP 的 whoami 核对身份和工作视图，再 get_board 读取项目路径、列规则和名册。
身份或工作视图不匹配时停止，报告实际情况，不确认成功。
遵守看板规则和 slate-kanban skill 的领取、派发、审查、交接流程。监听器负责所有事件游标。
本轮跳过 watch_prompt 中的守候循环，严禁调用 wait_for_updates 或 pull_updates，处理完退出。
收到的事件标识是 {json.dumps(changes, ensure_ascii=False)}。
对账模式为 {bool(reconcile)}。启用时核对现有领取、可领队列和卡片最新状态。
恢复原有领取和子 agent 前核对卡片、分支和派发记录。需要时用 get_card 读取相关卡片，按空闲名额调度。
需要人决定时在看板留言，不触发终端阻塞提问。
等本轮派出的子 agent 完成并完成交接，再检查空出的名额是否有后续工作。不要留下需要本进程跟进的子 agent。
必要的留言和交接已写入、领取已按流程处置且子 agent 已结束时，本轮可确认已处理。卡片仍等待人工补验、依赖未解决或暂无可领任务，都可以是本轮已处理的结果。卡片继续遵守原列规则。
{receipt}"""


def dispatch_invocation(host, executable, url, prompt, output, schema=None):
    if host == "codex":
        command = [
            *executable, "exec", "--output-last-message", str(output),
            "-c", "mcp_servers.slate.url=" + json.dumps(url),
        ]
        if schema is not None:
            command.extend(["--output-schema", str(schema)])
        if os.environ.get(PASSCODE_ENV):
            command.extend(["-c", f'mcp_servers.slate.bearer_token_env_var="{PASSCODE_ENV}"'])
        return [*command, "-"], prompt
    if host == "claude":
        return [*executable, "--print", "--output-format", "json"], prompt
    if host == "cursor":
        return [*executable, "--print", "--output-format", "stream-json", prompt], None
    if host == "opencode":
        return [*executable, "run", "--format", "json", prompt], None
    if host in ("pi", "on-my-pi"):
        return [*executable, "--print", "--mode", "text", prompt], None
    return executable, prompt


def receipt_schema(path):
    path.write_text(json.dumps({
        "type": "object",
        "properties": {
            "dispatch_id": {"type": "string"},
            "outcome": {"type": "string", "enum": RECEIPT_OUTCOMES},
        },
        "required": ["dispatch_id", "outcome"],
        "additionalProperties": False,
    }), encoding="utf-8")
    return path


def unique_json_object(pairs):
    result = dict(pairs)
    if len(result) != len(pairs):
        raise ValueError("Duplicate receipt keys")
    return result


def validate_receipt(output, dispatch_id):
    data = output.read(8193)
    if len(data) > 8192:
        raise DispatchFailure("receipt_invalid")
    if not data.strip():
        raise DispatchFailure("receipt_missing")
    try:
        reply = json.loads(data, object_pairs_hook=unique_json_object)
    except (ValueError, RecursionError):
        raise DispatchFailure("receipt_invalid") from None
    if (not isinstance(reply, dict) or set(reply) != {"dispatch_id", "outcome"} or
            not isinstance(reply["dispatch_id"], str) or reply["outcome"] not in RECEIPT_OUTCOMES):
        raise DispatchFailure("receipt_invalid")
    if reply["dispatch_id"] != dispatch_id:
        raise DispatchFailure("receipt_dispatch_mismatch")
    if reply["outcome"] == "unconfirmed":
        raise DispatchFailure("agent_unconfirmed")


def cursor_dispatch_succeeded(output):
    # Cursor's terminal result concatenates earlier assistant progress and
    # the final reply. Use the complete assistant messages in stream-json.
    session = None
    final_marker = False
    finished = False
    pending_calls = set()
    for line in output:
        if not line.strip():
            continue
        event = json.loads(line)
        if finished or not isinstance(event, dict):
            return False
        kind = event.get("type")
        if session is None:
            session = event.get("session_id")
            if (kind != "system" or event.get("subtype") != "init" or
                    not isinstance(session, str) or not session):
                return False
            continue
        if event.get("session_id") != session or kind in ("error", "system"):
            return False
        if kind == "result":
            finished = (
                final_marker and not pending_calls and event.get("subtype") == "success" and
                event.get("is_error") is False and isinstance(event.get("result"), str)
            )
            if not finished:
                return False
            continue

        final_marker = False
        if kind == "assistant":
            message = event.get("message")
            if not isinstance(message, dict) or message.get("role") != "assistant":
                return False
            content = message.get("content")
            if isinstance(content, list) and all(
                isinstance(part, dict) and part.get("type") == "text" and isinstance(part.get("text"), str)
                for part in content
            ) and "timestamp_ms" not in event and "model_call_id" not in event:
                final_marker = "".join(part["text"] for part in content).strip() == DONE
        elif kind == "tool_call":
            call_id = event.get("call_id")
            subtype = event.get("subtype")
            if not isinstance(call_id, str) or not call_id:
                return False
            if subtype == "started" and call_id not in pending_calls:
                pending_calls.add(call_id)
            elif subtype == "completed" and call_id in pending_calls:
                pending_calls.remove(call_id)
            else:
                return False
    return finished


def dispatch_succeeded(host, output):
    if host == "cursor":
        return cursor_dispatch_succeeded(output)
    if host == "opencode":
        # Only a finished final assistant step can confirm a batch. Ignore tool output.
        answer = None
        parts = []
        for line in output:
            if not line.strip():
                continue
            event = json.loads(line)
            if not isinstance(event, dict) or event.get("type") == "error":
                return False
            kind = event.get("type")
            part = event.get("part")
            if kind == "step_start":
                answer = None
                parts = []
            elif kind == "text" and isinstance(part, dict) and isinstance(part.get("text"), str):
                parts.append(part["text"])
            elif kind == "step_finish" and isinstance(part, dict):
                answer = "\n".join(parts).strip() if part.get("reason") == "stop" else None
                parts = []
        return answer == DONE and not parts
    if host == "claude":
        result = json.load(output)
        return (
            isinstance(result, dict) and result.get("type") == "result" and
            result.get("subtype") == "success" and result.get("is_error") is False and
            isinstance(result.get("result"), str) and result["result"].strip() == DONE
        )
    return output.read(8192).strip() == DONE and not output.read(1)


def dispatch_agent(args, url, host, executable, events, reconcile):
    structured = host == "codex" and not getattr(args, "codex_legacy_receipt", False)
    dispatch_id = uuid4().hex if structured else None
    prompt = dispatch_prompt(args, events, reconcile, dispatch_id)
    with tempfile.TemporaryDirectory(prefix="slate-dispatch-") as directory:
        root = Path(directory)
        result = root / "result.txt"
        if structured:
            schema = receipt_schema(root / "receipt-schema.json")
        else:
            schema = None
        command, stdin = dispatch_invocation(host, executable, url, prompt, result, schema)
        env = {
            **os.environ,
            "SLATE_WATCH_URL": url,
            "SLATE_WATCH_BOARD": str(args.board),
            "SLATE_WATCH_NAME": args.name,
        }
        with tempfile.TemporaryFile() as stdout:
            input_options = {"input": stdin.encode("utf-8")} if stdin is not None else {"stdin": subprocess.DEVNULL}
            try:
                completed = subprocess.run(
                    command, **input_options, cwd=args.project, env=env,
                    stdout=stdout, stderr=subprocess.DEVNULL, check=False,
                )
            except (OSError, ValueError):
                raise DispatchFailure("host_launch_failed") from None
            if completed.returncode != 0:
                raise DispatchFailure(f"host_exit ({completed.returncode})")

            try:
                if host == "codex":
                    output = result.open(encoding="utf-8")
                else:
                    stdout.seek(0)
                    output = open(stdout.fileno(), encoding="utf-8", closefd=False)
                with output:
                    if structured:
                        validate_receipt(output, dispatch_id)
                        return
                    if dispatch_succeeded(host, output):
                        return
            except FileNotFoundError:
                raise DispatchFailure("receipt_missing") from None
            except (OSError, UnicodeError):
                raise DispatchFailure("receipt_unreadable") from None
            except (ValueError, RecursionError):
                raise DispatchFailure("receipt_invalid") from None
            raise DispatchFailure("receipt_missing")


def run_worker(args, url, state):
    host, executable = resolve_dispatcher(args)
    if args.codex_legacy_receipt and host != "codex":
        raise PendingDispatch("--codex-legacy-receipt requires Codex")
    client = SlateMCP(url)
    backoff = 1
    more = False

    def dispatch(events, reconcile):
        dispatch_agent(args, url, host, executable, events, reconcile)

    while True:
        try:
            if not client.session:
                client.connect()
            more = watch_once(state, client, dispatch, args.board, args.timeout, more=more, host=host)
            backoff = 1
        except HTTPError as exc:
            exc.close()
            if 300 <= exc.code < 500 and exc.code not in (404, 408, 429):
                raise PendingDispatch(f"Slate MCP HTTP {exc.code}; check the address and access credentials") from exc
            print(f"Slate MCP HTTP {exc.code}; reconnecting in {backoff}s", file=sys.stderr)
        except (URLError, TimeoutError, ConnectionError, HTTPException, MCPError, ValueError) as exc:
            print(f"Slate MCP connection failed ({type(exc).__name__}); reconnecting in {backoff}s", file=sys.stderr)
        else:
            continue
        client.session = None
        time.sleep(backoff)
        backoff = min(backoff * 2, 30)


def supervise():
    while True:
        worker = subprocess.Popen([sys.executable, str(Path(__file__).resolve()), *sys.argv[1:], "--worker"])
        try:
            status = worker.wait()
        except KeyboardInterrupt:
            # Ctrl+C reaches the worker and its dispatcher child in the same console group.
            try:
                worker.wait(timeout=5)
            except (subprocess.TimeoutExpired, KeyboardInterrupt):
                worker.terminate()
                worker.wait()
            raise
        if status in (0, NEEDS_ATTENTION, 130):
            return status
        print(f"Slate listener exited ({status}); restarting in 5 seconds", file=sys.stderr)
        time.sleep(5)


def write_private(path, data):
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as handle:
        temporary = Path(handle.name)
        try:
            handle.write(data)
            handle.close()
            temporary.replace(path)
        finally:
            temporary.unlink(missing_ok=True)


class MacSupervisor:
    def __init__(self, identity, directory=None):
        self.domain = f"gui/{os.getuid()}"
        self.label = "dev.slate.kanban.watch." + identity
        self.directory = directory or Path.home() / "Library" / "Application Support" / "Slate" / "Watch" / identity
        self.profile = self.directory / "profile.json"
        self.report = self.directory / "health.json"
        self.agents = Path.home() / "Library" / "LaunchAgents"

    def job(self, kind):
        return f"{self.label}.{kind}"

    def plist(self, kind):
        return self.agents / (self.job(kind) + ".plist")

    def launchctl(self, command, *arguments, missing_ok=False):
        try:
            result = subprocess.run(["/bin/launchctl", command, *arguments], capture_output=True, timeout=15)
        except subprocess.TimeoutExpired:
            raise PendingDispatch(f"launchctl {command} timed out for {self.label}") from None
        if result.returncode == 113 and missing_ok:  # launchctl uses 113 for a missing service.
            return False
        if result.returncode:
            raise PendingDispatch(f"launchctl {command} failed ({result.returncode}) for {self.label}; "
                                  "check the macOS GUI login session and LaunchAgent permissions")
        return True

    def loaded(self, kind):
        return self.launchctl("print", self.domain + "/" + self.job(kind), missing_ok=True)

    def write_job(self, kind, log):
        job = {
            "Label": self.job(kind),
            "ProgramArguments": [sys.executable, str(Path(__file__).resolve()),
                                 "--_supervisor-" + kind, str(self.profile)],
            "StandardErrorPath": str(self.directory / log),
        }
        if kind == "check":
            job.update(RunAtLoad=True, StartInterval=SUPERVISOR_INTERVAL)
        write_private(self.plist(kind), plistlib.dumps(job))

    def enable(self, args, path):
        if (self.profile.exists() or self.plist("listener").exists() or self.plist("check").exists()
                or self.loaded("listener") or self.loaded("check")):
            raise PendingDispatch("A supervisor already exists for this identity. Check --supervise status; "
                                  "use --supervise disable before changing its settings.")
        resolve_dispatcher(args)
        arguments = ["--url", args.url, "--board", str(args.board), "--name", args.name,
                     "--project", str(args.project.resolve()), "--state-dir", str(path.parent),
                     "--host", args.host, "--timeout", str(args.timeout)]
        if args.command_json:
            arguments.extend(("--command-json", json.dumps(args.command_json)))
        if args.codex:
            arguments.extend(("--codex", args.codex))
        if args.codex_legacy_receipt:
            arguments.append("--codex-legacy-receipt")
        profile = {"arguments": arguments, "state_path": str(path), "board": args.board,
                   "name": args.name, "directory": str(Path.cwd()),
                   "path": os.environ.get("PATH", os.defpath), "passcode": os.environ.get(PASSCODE_ENV)}
        self.agents.mkdir(parents=True, exist_ok=True)
        write_private(self.profile, json.dumps(profile).encode())
        try:
            for kind in ("listener", "check"):
                self.write_job(kind, kind + ".log")
                self.launchctl("bootstrap", self.domain, str(self.plist(kind)))
        except (PendingDispatch, OSError) as error:
            try:
                self.disable()
            except (PendingDispatch, OSError):
                raise PendingDispatch(f"{error}. Partial installation remains; use --supervise disable.") from None
            raise

    def disable(self):
        for kind in ("check", "listener"):
            if self.loaded(kind):
                self.launchctl("bootout", self.domain + "/" + self.job(kind))
            self.plist(kind).unlink(missing_ok=True)
        self.profile.unlink(missing_ok=True)
        self.report.unlink(missing_ok=True)

    def verify_state_path(self, path):
        try:
            profile = json.loads(self.profile.read_text())
        except FileNotFoundError:
            return
        if Path(profile["state_path"]) != path:
            raise PendingDispatch("The installed supervisor uses a different --state-dir for this identity")

    def record(self, health, attempts, detail=None):
        data = {"health": health, "starts_without_healthy_check": attempts,
                "checked_at": int(time.time()), "detail": detail}
        write_private(self.report, json.dumps(data).encode())

    def last_check(self):
        if self.report.exists():
            return json.loads(self.report.read_text())
        return {}

    def check(self, args, path):
        path.parent.mkdir(parents=True, exist_ok=True)
        try:
            with listener_lock(path.with_suffix(".lock")):
                status = saved_status(args, path)
        except ListenerBusy:
            self.record("running", 0, "Identity lock is held; skipped starting another listener.")
            return
        if status["pending_cursor"] is not None:
            self.record("pending_dispatch", 0, status["failure"] or "outcome_not_recorded")
            return
        attempts = self.last_check().get("starts_without_healthy_check", 0)
        if attempts >= SUPERVISOR_MAX_STARTS:
            self.record("start_failures", attempts,
                        "Inspect listener.log; use --supervise disable and enable after fixing the cause.")
            return
        self.record("starting", attempts + 1)
        try:
            self.launchctl("kickstart", self.domain + "/" + self.job("listener"))
        except PendingDispatch as error:
            self.record("start_failed", attempts + 1, str(error))

    def status(self, args, path):
        self.verify_state_path(path)
        status = saved_status(args, path)
        status.update({"check_loaded": self.loaded("check"), "listener_loaded": self.loaded("listener"),
                       "identity_in_use": identity_in_use(path),
                       "label": self.label, "last_check": self.last_check(),
                       "log_directory": str(self.directory)})
        print(json.dumps(status))


def supervisor_command(args, identity, path):
    if sys.platform != "darwin":
        raise PendingDispatch("--supervise is available in a macOS GUI login session")
    supervisor = MacSupervisor(identity)
    supervisor.launchctl("print", supervisor.domain)
    if args.supervise == "status":
        supervisor.status(args, path)
        return 0
    supervisor.directory.mkdir(parents=True, mode=0o700, exist_ok=True)
    with listener_lock(supervisor.directory / "manage.lock"):
        if args.supervise == "enable":
            supervisor.enable(args, path)
        else:
            supervisor.verify_state_path(path)
            supervisor.disable()
    supervisor.status(args, path)
    return 0


def supervisor_job(kind, profile_path):
    profile = json.loads(profile_path.read_text())
    path = Path(profile["state_path"])
    if kind == "check":
        supervisor = MacSupervisor(path.stem, profile_path.parent)
        supervisor.check(argparse.Namespace(board=profile["board"], name=profile["name"]), path)
        return 0
    os.chdir(profile["directory"])
    os.environ["PATH"] = profile["path"]
    os.environ.pop(PASSCODE_ENV, None)
    if profile["passcode"]:
        os.environ[PASSCODE_ENV] = profile["passcode"]
    return main([*profile["arguments"], "--worker"])


def main(argv=None):
    if argv is None:
        argv = sys.argv[1:]
    if len(argv) == 2 and argv[0] in ("--_supervisor-check", "--_supervisor-listener"):
        return supervisor_job(argv[0].removeprefix("--_supervisor-"), Path(argv[1]))
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", required=True, help="Slate /mcp URL; set SLATE_PASSCODE if required")
    parser.add_argument("--board", required=True, type=int, help="numeric board ID")
    parser.add_argument("--name", required=True, help="stable Slate work identity")
    parser.add_argument("--project", required=True, type=Path, help="board project directory")
    parser.add_argument("--timeout", default=50, type=int, help="MCP wait seconds, 1–3600")
    parser.add_argument("--state-dir", default=Path.home() / ".slate-kanban" / "watch", type=Path)
    parser.add_argument("--host", default="auto", choices=("auto", *HOST_EXECUTABLES, *HOST_ALIASES),
                        help="dispatcher type; auto uses the standard Slate name or a unique installed CLI")
    launcher = parser.add_mutually_exclusive_group()
    launcher.add_argument("--codex", help="legacy option for a specific Codex executable")
    launcher.add_argument("--command-json", type=command_argv,
                          help="local non-interactive argv as JSON; reads stdin and emits only the final reply")
    parser.add_argument("--codex-legacy-receipt", action="store_true",
                        help="use the exact legacy marker for a Codex CLI without --output-schema")
    parser.add_argument("--worker", action="store_true", help=argparse.SUPPRESS)
    recovery = parser.add_mutually_exclusive_group()
    recovery.add_argument("--status", action="store_true", help="read saved listener state without starting a dispatcher")
    recovery.add_argument("--supervise", choices=("enable", "status", "disable"),
                          help="manage a macOS LaunchAgent supervisor for this listener identity")
    recovery.add_argument(
        "--resolve", choices=("done", "retry"),
        help="after verifying a stopped dispatch, mark handled or explicitly permit retry",
    )
    args = parser.parse_args(argv)
    args.host = HOST_ALIASES.get(args.host, args.host)
    if args.command_json and args.host != "auto":
        parser.error("choose --host or --command-json")
    if args.codex and args.host not in ("auto", "codex"):
        parser.error("--codex requires --host codex or auto")
    if args.codex_legacy_receipt and (args.command_json or args.host not in ("auto", "codex")):
        parser.error("--codex-legacy-receipt requires Codex")
    if args.board < 1 or not 1 <= args.timeout <= 3600 or not re.fullmatch(r"[a-z0-9][a-z0-9._-]{0,63}", args.name):
        parser.error("use a positive board ID, Slate work name and timeout between 1 and 3600")
    endpoint = urlsplit(args.url)
    if endpoint.scheme not in ("http", "https") or not endpoint.hostname or endpoint.username or endpoint.fragment:
        parser.error("--url must be an HTTP(S) MCP endpoint without credentials or fragment")
    query = dict(parse_qsl(endpoint.query))
    query.pop("role", None)
    query["name"] = args.name
    url = urlunsplit(endpoint._replace(query=urlencode(sorted(query.items()))))
    if not (args.resolve or args.status or args.supervise in ("status", "disable")) and not args.project.is_dir():
        parser.error("--project must be an existing directory")
    identity = hashlib.sha256(json.dumps([url, args.board]).encode()).hexdigest()
    path = args.state_dir.resolve() / (identity + ".sqlite3")
    if args.supervise:
        return supervisor_command(args, identity, path)
    if not (args.worker or args.resolve or args.status):
        return supervise()
    if args.status:
        print(json.dumps(saved_status(args, path)))
        return 0
    args.state_dir.mkdir(parents=True, exist_ok=True)
    with listener_lock(path.with_suffix(".lock")):
        state = WatchState(path)
        try:
            if args.resolve:
                state.resolve(args.resolve)
                return 0
            _, _, pending = state.status()
            if pending is not None:
                raise PendingDispatch(pending_message(state, pending))
            run_worker(args, url, state)
        except PendingDispatch as error:
            raise PendingDispatch(f"Slate board {args.board}, identity {args.name}: {error}") from None
        finally:
            state.close()


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (PendingDispatch, OSError, sqlite3.Error) as error:
        print(error, file=sys.stderr)
        sys.exit(NEEDS_ATTENTION)
    except KeyboardInterrupt:
        sys.exit(130)
