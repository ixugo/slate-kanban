"""Measure Slate MCP with a fresh SQLite database and fixed synthetic tasks."""

import argparse
from contextlib import contextmanager
from concurrent.futures import ThreadPoolExecutor
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
from threading import Thread
import time
from urllib.error import URLError
from urllib.request import Request, urlopen

import tiktoken


def compact(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def exchange(url, payload=None, headers=None):
    request = Request(
        url, data=compact(payload).encode() if payload is not None else None,
        headers={"Content-Type": "application/json", **(headers or {})},
    )
    with urlopen(request, timeout=8) as response:
        return response.headers, response.read().decode()


@contextmanager
def serve(binary):
    with tempfile.TemporaryDirectory(prefix="slate-mcp-measure-") as directory:
        config = Path(directory)
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            port = sock.getsockname()[1]
        base = f"http://127.0.0.1:{port}"
        (config / "config.toml").write_text(f"""
[Server.HTTP]
Host = "127.0.0.1"
Port = {port}
[Data.Database]
Dsn = {compact(str(config / "data.db"))}
[Log]
Dir = {compact(str(config / "logs"))}
""")
        with (config / "server.log").open("w+") as log:
            process = subprocess.Popen([binary.resolve(), "-conf", config], stdout=log, stderr=log)
            try:
                for _ in range(100):
                    if process.poll() is not None:
                        break
                    try:
                        exchange(base + "/api/v1/boards")
                    except URLError:
                        time.sleep(0.1)
                    else:
                        yield base
                        return
                log.seek(0)
                raise RuntimeError("Slate failed to start:\n" + log.read()[-4000:])
            finally:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


class MCP:
    def __init__(self, url, counter):
        self.url = url
        self.count = counter
        self.headers = {"Accept": "application/json, text/event-stream"}
        self.seq = 0
        self.measured = {}
        reply = self.request("initialize", {
            "protocolVersion": "2025-06-18", "capabilities": {},
            "clientInfo": {"name": "slate-mcp-token-measure", "version": "1"},
        })
        self.instructions = reply.get("instructions", "")
        self.version = reply["serverInfo"]["version"]
        self.headers["MCP-Protocol-Version"] = reply["protocolVersion"]
        exchange(self.url, {"jsonrpc": "2.0", "method": "notifications/initialized"}, self.headers)

    def request(self, method, params, label=None):
        self.seq += 1
        payload = {"jsonrpc": "2.0", "id": self.seq, "method": method, "params": params}
        headers, wire = exchange(self.url, payload, self.headers)
        if session := headers.get("Mcp-Session-Id"):
            self.headers["Mcp-Session-Id"] = session
        if wire.startswith("event:"):
            wire = next(line[6:] for line in wire.splitlines() if line.startswith("data: "))
        value = json.loads(wire)
        if "error" in value:
            raise RuntimeError(f"{method}: {value['error']}")
        result = value["result"]
        if result.get("isError"):
            raise RuntimeError(f"{method}: {result['content']}")
        row = {"request": params, "request_tokens": self.count(compact(payload)),
               "reply_tokens": self.count(wire)}
        if "structuredContent" in result:
            row["structured_tokens"] = self.count(compact(result["structuredContent"]))
        self.measured[label or method] = row
        return result

    def call(self, tool, label=None, **args):
        reply = self.request("tools/call", {"name": tool, "arguments": args}, label or tool)
        return reply["structuredContent"]


def discovery(client):
    tools = client.request("tools/list", {})["tools"]
    assert len(tools) == len({tool["name"] for tool in tools})
    rows = []
    for tool in tools:
        row = {"name": tool["name"], "name_tokens": client.count(tool["name"]),
               "description_tokens": client.count(tool.get("description", "")),
               "schema_tokens": client.count(compact(tool.get("inputSchema", {}))),
               "tool_tokens": client.count(compact(tool))}
        rows.append(row)
    total = sum(row["tool_tokens"] for row in rows)
    for row in rows:
        row["share_percent"] = round(100 * row["tool_tokens"] / total, 2)
    return {"service_version": client.version, "tool_count": len(tools),
            "instructions_tokens": client.count(client.instructions), "tools": rows,
            **client.measured}


def rest(base, path, payload):
    _, wire = exchange(base + "/api/v1" + path, payload)
    result = json.loads(wire)
    if result.get("code", 0) != 0:
        raise RuntimeError(f"Fixture {path}: {result}")
    return result.get("data", result)


def measure(base, count, compact_boards=False, discovery_only=False):
    worker = MCP(base + "/mcp?name=measure-worker", count)
    legacy = MCP(base + "/mcp?name=measure-manager&role=manager", count)
    report = {"work_discovery": discovery(worker), "legacy_management_discovery": discovery(legacy)}
    tool_names = {row["name"] for view in report.values() for row in view["tools"]}
    if any(row["name"] == "enable_management" for row in report["work_discovery"]["tools"]):
        mgmt = MCP(base + "/mcp?name=measure-management", count)
        mgmt.call("enable_management")
        report["on_demand_management_discovery"] = discovery(mgmt)
        tool_names.update(row["name"] for row in report["on_demand_management_discovery"]["tools"])
    report["unique_tools_across_views"] = len(tool_names)
    if discovery_only:
        return report

    empty = rest(base, "/boards", {"name": "Empty fixture", "slug": "EMPTY", "template": "dev"})
    board = rest(base, "/boards", {"name": "工作样例 / Work fixture", "slug": "SAMPLE", "template": "dev"})
    board_id = board["id"]
    board_path = f"/boards/{board_id}"
    inbox, todo, doing, review, done = (column["id"] for column in board["columns"])

    def card(title, description, column, **fields):
        return rest(base, board_path + "/cards", {
            "title": title, "description": description, "column_id": column, **fields,
        })["id"]

    first_dep = card("已完成的依赖 A", "完成的依赖。", done)
    second_dep = card("尚未完成的依赖 B", "知识库同步。", inbox)
    typical = card("校验消息交接 / Validate handoff", "逐项检查交接记录、任务状态和租约。", doing)
    long_desc = ("验收时请检查消息顺序、重试、权限和审查记录。 Validate order, retries and review.\n" * 30)
    long_id = card("多评论的长期任务 / Long task", long_desc, todo, waits_on=[first_dep, second_dep])
    for i in range(1, 6):
        rest(base, f"/cards/{long_id}/comments", {
            "body": f"第 {i} 条验收补充：记录命令、消息和回滚步骤。 Include commands, messages and rollback.",
        })
    report["fixture"] = {"boards": 2, "work_cards": 4, "long_description_characters": len(long_desc),
                         "long_comments": 5, "long_dependencies": 2, "language": "Chinese and English"}

    worker.measured = {}
    worker.call("list_boards", **({"summary": True} if compact_boards else {}))
    worker.call("get_board", board_id=board_id, **({"include_watch_prompt": False} if compact_boards else {}))
    worker.call("list_cards", board_id=board_id, query=f"col:{doing} is:free -is:waiting -is:human")
    worker.call("get_card", "get_card_typical", card_id=typical)
    claimed = worker.call("next_card", board_id=board_id, column_id=doing)
    assert claimed["card"]["id"] == typical
    worker.call("comment_card", card_id=typical, body="研发自测通过。")
    finished = worker.call("finish_card", card_id=typical, branch="slate/sample", summary="已交付，请审查交接。")
    assert finished["column_id"] == review
    report["dispatch"] = {"calls": len(worker.measured),
                          "requests_tokens": sum(row["request_tokens"] for row in worker.measured.values()),
                          "replies_tokens": sum(row["reply_tokens"] for row in worker.measured.values())}

    worker.call("get_board", "get_board_empty", board_id=empty["id"])
    worker.call("list_cards", "list_cards_empty", board_id=empty["id"])
    worker.call("list_cards", "list_cards_unfiltered", board_id=board_id)
    worker.call("get_card", "get_card_long", card_id=long_id)
    worker.call("next_card", "next_card_empty", board_id=empty["id"], column_id=empty["columns"][2]["id"])
    first = worker.call("wait_for_updates", "wait_first", board_id=board_id, timeout_seconds=1)
    assert first["first"]
    since = first["cursor"]
    idle = worker.call("wait_for_updates", "wait_empty", board_id=board_id, since=since, timeout_seconds=1)
    assert idle["timed_out"] and not idle["events"]
    with ThreadPoolExecutor(max_workers=1) as pool:
        pending = pool.submit(worker.call, "wait_for_updates", "wait_event",
                              board_id=board_id, since=idle["cursor"], timeout_seconds=3)
        time.sleep(0.2)
        rest(base, f"/cards/{typical}/comments", {"body": "请补交接验证记录。"})
        event = pending.result()
    assert any(item["type"] == "comment.added" for item in event["events"])
    report["tool_calls"] = worker.measured
    return report


def measure_codex(base, name, route, count, *, management=False):
    # A synthetic Responses endpoint drives real MCP tool calls. Capture only Slate
    # entries, keeping unrelated instructions, tools and headers out of reports.
    captures = []
    column_name = "Codex compatibility sample"
    if management:
        board = rest(base, "/boards", {"name": "Host compatibility", "slug": "HOST", "template": "dev"})

    class Capture(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            namespace = next((tool for tool in body["tools"] if tool.get("name") == "mcp__slate"), {})
            functions = namespace.get("tools", [])
            outputs = [item for item in body["input"] if item.get("type") == "function_call_output"]
            captures.append({
                "model": body["model"], "slate_namespace_tokens": count(compact(namespace)),
                "namespace_instruction_tokens": count(namespace.get("description", "")),
                "tool_count": len(functions),
                "tools_with_full_instructions": sum(
                    bool(namespace.get("description")) and namespace["description"] in tool.get("description", "")
                    for tool in functions
                ),
                "tool_output_tokens": count(compact(outputs[-1])) if outputs else None,
                "management_tool_visible": any(tool.get("name") == "create_column" for tool in functions),
            })
            if management and len(outputs) == 1:
                # The SDK sends list-change notifications asynchronously.
                time.sleep(0.3)

            if len(outputs) >= (2 if management else 1):
                item = {"type": "message", "id": "msg_capture", "role": "assistant", "status": "completed",
                        "content": [{"type": "output_text", "text": "Done.", "annotations": []}]}
            else:
                tool = "enable_management" if management else "whoami"
                arguments = {}
                if management:
                    arguments = ({"include_tools": True} if not outputs else {
                        "tool": "create_column", "arguments": {"board_id": board["id"], "name": column_name},
                    })
                step = "compatibility" if management and outputs else tool
                item = {"type": "function_call", "id": f"fc_{step}", "call_id": f"call_{step}",
                        "namespace": "mcp__slate", "name": tool, "arguments": compact(arguments), "status": "completed"}
            events = [
                {"type": "response.output_item.added", "output_index": 0, "item": item},
                {"type": "response.output_item.done", "output_index": 0, "item": item},
                {"type": "response.completed", "response": {
                    "id": f"resp_capture_{len(captures)}", "object": "response",
                    "created_at": 0, "status": "completed", "model": body["model"], "output": [item],
                }},
            ]
            stream = "".join(f"event: {event['type']}\ndata: {compact(event)}\n\n" for event in events)
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self.wfile.write(stream.encode())

    with ThreadingHTTPServer(("127.0.0.1", 0), Capture) as server, tempfile.TemporaryDirectory() as cwd:
        thread = Thread(target=server.serve_forever, daemon=True)
        thread.start()
        endpoint = f"http://127.0.0.1:{server.server_port}/v1"
        try:
            command = [name, "exec", "--ignore-user-config", "--ephemeral", "--skip-git-repo-check",
                       "--json", "-C", cwd, "--sandbox", "read-only", "-m", "gpt-5.4"]
            settings = {
                "model_provider": "slate_capture", "model_providers.slate_capture.name": "slate capture",
                "model_providers.slate_capture.base_url": endpoint,
                "model_providers.slate_capture.env_key": "SLATE_CAPTURE_BEARER",
                "model_providers.slate_capture.wire_api": "responses",
                "mcp_servers.slate.url": base + route,
            }
            for key, value in settings.items():
                command.extend(("-c", f"{key}={compact(value)}"))
            prompt = "Manage Slate board settings." if management else "Call Slate whoami."
            try:
                process = subprocess.run(
                    [*command, prompt], env=os.environ | {"SLATE_CAPTURE_BEARER": "synthetic-local"},
                    capture_output=True, text=True, timeout=45,
                )
            except subprocess.TimeoutExpired as error:
                raise RuntimeError(f"Codex capture timed out after {len(captures)} local Responses requests") from error
            if process.returncode or not captures or captures[-1]["tool_output_tokens"] is None:
                raise RuntimeError(f"Codex capture did not complete the local tool turn ({process.returncode}, {len(captures)} requests)")
        finally:
            server.shutdown()
            thread.join()
    if management:
        confirmed = rest(base, f"/boards/{board['id']}", None)
        assert any(column["name"] == column_name for column in confirmed["columns"])
        return {"initial_request": captures[0], "after_enable": captures[1], "after_compatibility_call": captures[-1],
                "compatibility_call_created_column": True}
    return {"initial_request": captures[0], "after_whoami": captures[-1]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path, help="Slate binary to start on loopback")
    parser.add_argument("--codex", help="Codex CLI binary for optional local Responses capture")
    parser.add_argument("--discovery-only", action="store_true", help="skip dispatch and event wait samples")
    args = parser.parse_args()
    encoding = tiktoken.get_encoding("o200k_base")

    def count(text):
        return len(encoding.encode_ordinary(text))

    with serve(args.binary) as base:
        report = measure(base, count, discovery_only=args.discovery_only)
        if args.codex:
            report["codex"] = {
                "host": subprocess.check_output([args.codex, "--version"], text=True).strip(),
                "work": measure_codex(base, args.codex, "/mcp?name=measure-host", count),
                "legacy_management": measure_codex(base, args.codex, "/mcp?name=measure-host-manager&role=manager", count),
            }
            if "on_demand_management_discovery" in report:
                report["codex"]["on_demand"] = measure_codex(
                    base, args.codex, "/mcp?name=measure-host-management", count, management=True,
                )
    if not args.discovery_only and "on_demand_management_discovery" in report:
        with serve(args.binary) as base:
            compact_report = measure(base, count, compact_boards=True)
        report["compact_board_dispatch"] = {
            "dispatch": compact_report["dispatch"],
            "list_boards": compact_report["tool_calls"]["list_boards"],
            "get_board": compact_report["tool_calls"]["get_board"],
        }
    print(json.dumps({"tokenizer": encoding.name, "tokenizer_version": tiktoken.__version__, **report},
                     ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
