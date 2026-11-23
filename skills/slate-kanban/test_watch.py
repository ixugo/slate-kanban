from contextlib import closing
import hashlib
import io
import json
import os
import plistlib
import re
import sqlite3
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

from watch import (
    DONE, MacSupervisor, PendingDispatch, WatchState, command_argv, dispatch_agent, dispatch_succeeded,
    handle_batch, identity_in_use, listener_lock, resolve_dispatcher, watch_once,
)


def updates(cursor=42, *, events=(), **flags):
    return {"cursor": cursor, "events": list(events), **flags}


EVENT = {"id": 42, "type": "card.moved", "card_id": 5}


class WatchTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.path = Path(tmp.name) / "watch.sqlite3"
        self.state = WatchState(self.path)
        self.addCleanup(self.state.close)
        self.mcp = Mock()
        self.dispatch = Mock()

    def test_empty_timeouts_never_launch_dispatcher(self):
        self.state.processed(41)
        self.mcp.tool.return_value = updates(42, timed_out=True)
        for _ in range(3):
            watch_once(self.state, self.mcp, self.dispatch, 1, 5)
        self.dispatch.assert_not_called()
        self.mcp.tool.assert_called_with(
            "wait_for_updates", {"board_id": 1, "since": 42, "timeout_seconds": 5}, 15
        )

    def test_processed_batch_is_acknowledged_after_restart_without_dispatch(self):
        self.state.processed(41)
        self.mcp.tool.return_value = updates(events=[EVENT])
        watch_once(self.state, self.mcp, self.dispatch, 1, 5)
        self.assertEqual(self.mcp.tool.call_args.args[1]["since"], 41)
        self.assertEqual(self.dispatch.call_count, 1)
        reopened = WatchState(self.path)
        self.addCleanup(reopened.close)
        self.mcp.tool.return_value = updates(timed_out=True)
        watch_once(reopened, self.mcp, self.dispatch, 1, 5)
        self.assertEqual(self.dispatch.call_count, 1)
        self.assertEqual(self.mcp.tool.call_args.args[1]["since"], 42)

    def test_dispatch_failure_blocks_restart_until_resolved(self):
        self.state.processed(41)
        self.dispatch.side_effect = RuntimeError("process failed")
        with self.assertRaises(RuntimeError):
            handle_batch(self.state, self.dispatch, updates(events=[EVENT]))
        reopened = WatchState(self.path)
        self.addCleanup(reopened.close)
        with self.assertRaises(PendingDispatch):
            watch_once(reopened, self.mcp, self.dispatch, 1, 5)
        self.mcp.tool.assert_not_called()
        self.assertEqual(self.dispatch.call_count, 1)
        reopened.resolve("done")
        self.assertEqual(reopened.cursor, 42)

    def test_reconnect_does_not_confirm_unreceived_event(self):
        self.state.processed(41)
        self.mcp.tool.side_effect = OSError("socket closed")
        with self.assertRaises(OSError):
            watch_once(self.state, self.mcp, self.dispatch, 1, 5)
        reconnected = Mock()
        reconnected.tool.return_value = updates(events=[EVENT])
        watch_once(self.state, reconnected, self.dispatch, 1, 5)
        self.assertEqual(self.mcp.tool.call_args.args, reconnected.tool.call_args.args)
        self.assertEqual(self.dispatch.call_count, 1)

    def test_first_response_loss_still_requires_bootstrap(self):
        self.mcp.tool.return_value = updates()
        watch_once(self.state, self.mcp, self.dispatch, 1, 5)
        self.assertEqual(self.mcp.tool.call_args.args[1], {"board_id": 1})
        self.assertTrue(self.dispatch.call_args.args[1])

    def test_truncated_requires_reconciliation(self):
        self.state.processed(41)
        handle_batch(self.state, self.dispatch, updates(truncated=True))
        self.assertTrue(self.dispatch.call_args.args[1])

    def test_old_database_preserves_pending_cursor_and_supports_both_resolutions(self):
        self.state.close()
        self.path.unlink()
        with sqlite3.connect(self.path) as old:
            old.executescript("""
                CREATE TABLE watch_state (id INTEGER PRIMARY KEY, bootstrapped INTEGER,
                    cursor INTEGER, pending_cursor INTEGER);
                INSERT INTO watch_state VALUES (1, 1, 41, 42);
            """)
        old.close()
        reopened = WatchState(self.path)
        self.addCleanup(reopened.close)
        self.assertEqual(reopened.dispatch_status(), (None, None))
        with self.assertRaisesRegex(PendingDispatch, "outcome_not_recorded"):
            watch_once(reopened, self.mcp, self.dispatch, 1, 5)
        reopened.resolve("retry")
        self.assertEqual(reopened.status(), (1, 41, None))
        self.mcp.tool.return_value = updates(events=[EVENT])
        watch_once(reopened, self.mcp, self.dispatch, 1, 5)
        self.mcp.tool.assert_called_once_with(
            "wait_for_updates", {"board_id": 1, "since": 41, "timeout_seconds": 5}, 15
        )
        reopened.pending(43)
        reopened.resolve("done")
        self.assertEqual(reopened.status(), (1, 43, None))

    def test_status_reads_failures_while_same_identity_lock_blocks_writes(self):
        with tempfile.TemporaryDirectory() as directory:
            url = "http://localhost/mcp?name=worker&token=hidden"
            identity = hashlib.sha256(json.dumps([url, 4]).encode()).hexdigest()
            path = Path(directory) / (identity + ".sqlite3")
            command = [sys.executable, str(Path(__file__).with_name("watch.py")), "--url", url,
                       "--board", "4", "--name", "worker", "--project", directory, "--state-dir", directory]
            with closing(WatchState(path)) as state:
                state.processed(41)
                state.pending(42, "codex")
                with listener_lock(path.with_suffix(".lock")):
                    active = subprocess.run([*command, "--status"], capture_output=True, text=True, check=True)
                    active_state = json.loads(active.stdout)
                    self.assertEqual((active_state["pending_cursor"], active_state["failure"]), (42, None))
                    state.dispatch_failed("host_exit (9)")
                    status = subprocess.run([*command, "--status"], capture_output=True, text=True, check=True)
                    duplicate = subprocess.run([*command, "--worker"], capture_output=True, text=True, check=False)
                    resolve = subprocess.run([*command, "--resolve", "done"], capture_output=True, text=True, check=False)
                self.assertEqual(duplicate.returncode, 2)
                self.assertEqual(resolve.returncode, 2)
                self.assertIn("Another listener owns this state file", duplicate.stderr)
                self.assertNotIn("hidden", status.stdout + status.stderr + duplicate.stderr + resolve.stderr)
                data = json.loads(status.stdout)
                self.assertEqual((data["board"], data["name"], data["host"]), (4, "worker", "codex"))
                self.assertEqual((data["cursor"], data["pending_cursor"], data["failure"]), (41, 42, "host_exit (9)"))
                self.assertIn("--resolve done", data["recovery"])
                self.assertEqual(state.status(), (1, 41, 42))


class MacSupervisorTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.directory = Path(directory.name)
        self.path = self.directory / "fixture.sqlite3"
        self.args = SimpleNamespace(board=4, name="worker")
        with patch("watch.os.getuid", return_value=501, create=True):
            self.supervisor = MacSupervisor("fixture", self.directory)
        self.supervisor.agents = self.directory
        self.supervisor.launchctl = Mock()

    def test_only_checker_has_native_five_minute_interval(self):
        for kind in ("check", "listener"):
            self.supervisor.write_job(kind, kind + ".log")
        checker = plistlib.loads(self.supervisor.plist("check").read_bytes())
        listener = plistlib.loads(self.supervisor.plist("listener").read_bytes())
        self.assertTrue(checker["RunAtLoad"])
        self.assertEqual(checker["StartInterval"], 300)
        for key in ("RunAtLoad", "StartInterval", "KeepAlive"):
            self.assertNotIn(key, listener)
        self.assertEqual(listener["ProgramArguments"][-2:],
                         ["--_supervisor-listener", str(self.supervisor.profile)])

    def test_checker_does_not_start_with_an_existing_lock_or_pending_cursor(self):
        lock = self.path.with_suffix(".lock")
        self.assertFalse(identity_in_use(self.path))
        self.assertFalse(lock.exists())
        with closing(WatchState(self.path)) as state:
            state.processed(41)
            state.pending(42)
            with listener_lock(lock):
                self.assertTrue(identity_in_use(self.path))
                self.supervisor.check(self.args, self.path)
                self.assertEqual(self.supervisor.last_check()["health"], "running")
            self.supervisor.check(self.args, self.path)
            self.assertEqual(self.supervisor.last_check()["health"], "pending_dispatch")
            self.assertEqual(state.status(), (1, 41, 42))
            self.supervisor.launchctl.assert_not_called()
            state.resolve("done")
            self.supervisor.check(self.args, self.path)
            self.assertEqual(state.status(), (1, 42, None))
        self.supervisor.launchctl.assert_called_once_with(
            "kickstart", "gui/501/dev.slate.kanban.watch.fixture.listener")

    def test_repeated_failures_stop_kickstarts_and_healthy_listener_resets_budget(self):
        self.supervisor.launchctl.side_effect = PendingDispatch("cannot start")
        for _ in range(4):
            self.supervisor.check(self.args, self.path)
        self.assertEqual(self.supervisor.last_check()["health"], "start_failures")
        self.assertEqual(self.supervisor.launchctl.call_count, 3)
        with listener_lock(self.path.with_suffix(".lock")):
            self.supervisor.check(self.args, self.path)
            self.assertEqual(self.supervisor.last_check()["starts_without_healthy_check"], 0)

    def test_disable_removes_jobs_and_keeps_listener_state(self):
        self.supervisor.profile.touch()
        for kind in ("check", "listener"):
            self.supervisor.write_job(kind, kind + ".log")
        with closing(WatchState(self.path)) as state:
            state.processed(41)
            self.supervisor.disable()
            self.assertEqual(state.status(), (1, 41, None))
        self.assertTrue(self.path.exists())
        self.assertFalse(self.supervisor.profile.exists())
        for kind in ("check", "listener"):
            self.assertFalse(self.supervisor.plist(kind).exists())

    def test_launcher_profile_keeps_credentials_out_of_launch_agent_arguments(self):
        self.supervisor.launchctl.return_value = False
        launch = SimpleNamespace(board=4, name="worker", host="auto", codex=None,
                                 command_json=["local-host"], codex_legacy_receipt=False,
                                 url="http://localhost/mcp", project=self.directory, timeout=5)
        with patch("watch.resolve_dispatcher"), patch.dict(os.environ, {"SLATE_PASSCODE": "fixture-secret"}):
            self.supervisor.enable(launch, self.path)
        self.assertEqual(json.loads(self.supervisor.profile.read_text())["passcode"], "fixture-secret")
        if os.name != "nt":
            self.assertEqual(stat.S_IMODE(self.supervisor.profile.stat().st_mode), 0o600)
        for kind in ("check", "listener"):
            self.assertNotIn(b"fixture-secret", self.supervisor.plist(kind).read_bytes())
        self.supervisor.verify_state_path(self.path)
        with self.assertRaisesRegex(PendingDispatch, "different --state-dir"):
            self.supervisor.verify_state_path(self.directory / "other.sqlite3")

    def test_partial_installation_unloads_own_jobs(self):
        self.supervisor.launchctl.side_effect = [False, False, True, PendingDispatch("bootstrap failed"), False, True, True]
        launch = SimpleNamespace(board=4, name="worker", host="auto", codex=None,
                                 command_json=None, codex_legacy_receipt=False,
                                 url="http://localhost/mcp", project=self.directory, timeout=5)
        with patch("watch.resolve_dispatcher"), self.assertRaisesRegex(PendingDispatch, "bootstrap failed"):
            self.supervisor.enable(launch, self.path)
        self.supervisor.launchctl.assert_called_with("bootout", "gui/501/dev.slate.kanban.watch.fixture.listener")
        self.assertFalse(self.supervisor.profile.exists())
        self.assertFalse(self.supervisor.plist("listener").exists())
        self.assertFalse(self.supervisor.plist("check").exists())


class DispatcherTests(unittest.TestCase):
    def dispatcher(self, name, installed, **options):
        args = SimpleNamespace(**({"name": name, "host": "auto", "codex": None, "command_json": None} | options))
        with patch("watch.shutil.which", side_effect=installed.get):
            return resolve_dispatcher(args)

    def test_auto_uses_standard_name_or_unique_installed_host(self):
        codex = str(Path.cwd() / "codex")
        pi = str(Path.cwd() / "pi")
        self.assertEqual(self.dispatcher("codex", {"codex": codex, "pi": pi}), ("codex", [codex]))
        self.assertEqual(self.dispatcher("team-a", {"pi": pi}), ("pi", [pi]))
        with self.assertRaisesRegex(PendingDispatch, "Set --host"):
            self.dispatcher("team-a", {"codex": codex, "pi": pi})
        with self.assertRaisesRegex(PendingDispatch, "Set --host"):
            self.dispatcher("team-a", {"agent": "agent"})
        self.assertEqual(self.dispatcher("cursor", {"agent": "agent"})[0], "cursor")

    def test_explicit_host_legacy_codex_and_custom_argv(self):
        omp = str(Path.cwd() / "omp")
        self.assertEqual(self.dispatcher("worker", {"omp": omp}, host="on-my-pi"), ("on-my-pi", [omp]))
        self.assertEqual(self.dispatcher("worker", {"my-codex": omp}, codex="my-codex"), ("codex", [omp]))
        command = command_argv('["wrapper", "--literal", "$HOME; echo untouched"]')
        self.assertEqual(
            self.dispatcher("worker", {"wrapper": omp}, command_json=command),
            ("custom", [omp, "--literal", "$HOME; echo untouched"]),
        )

    def test_open_code_does_not_confirm_tool_text_or_unfinished_steps(self):
        done = {"type": "text", "part": {"text": DONE}}
        stop = {"type": "step_finish", "part": {"reason": "stop"}}
        tool = {"type": "tool_use", "part": {"text": DONE}}
        error = {"type": "error", "error": {"message": "failure"}}
        def output(*sequence):
            return io.StringIO("\n".join(map(json.dumps, sequence)))

        for sequence in ([done], [tool, stop], [done, stop, error], [done, stop, done]):
            with self.subTest(sequence=sequence):
                self.assertFalse(dispatch_succeeded("opencode", output(*sequence)))
        self.assertTrue(dispatch_succeeded("opencode", output(done, stop)))

    def test_cursor_requires_final_assistant_and_successful_terminal_event(self):
        def output(*events):
            session = {"session_id": "test-session"}
            init = {"type": "system", "subtype": "init"}
            return io.StringIO("\n".join(json.dumps(session | event) for event in (init, *events)))

        def message(role, text):
            return {"type": role, "message": {"role": role, "content": [{"type": "text", "text": text}]}}

        done = message("assistant", DONE)
        progress = message("assistant", "Checking.")
        tool = {"type": "tool_call", "call_id": "call-1"}
        started = tool | {"subtype": "started"}
        completed = tool | {"subtype": "completed", "tool_call": {"result": {"content": DONE}}}
        stop = {"type": "result", "subtype": "success", "is_error": False, "result": "Checking." + DONE}
        self.assertTrue(dispatch_succeeded("cursor", output(progress, started, completed, done, stop)))

        invalid = {
            "no terminal event": (done,),
            "terminal before reply": (stop, done),
            "duplicate terminal": (done, stop, stop),
            "event after terminal": (done, stop, progress),
            "marker only in terminal aggregate": (progress, stop),
            "marker only in user prompt": (message("user", DONE), stop),
            "marker only in tool output": (started, completed, stop),
            "marker only before tool": (done, started, completed, stop),
            "marker only before later assistant text": (done, progress, stop),
            "incomplete tool call": (started, done, stop),
            "tool completion before start": (completed, started, done, stop),
            "mixed sessions": (done, stop | {"session_id": "another-session"}),
            "partial output": (done | {"timestamp_ms": 1}, stop),
            "wrong message role": (done | {"message": message("user", DONE)["message"]}, stop),
            "error event": ({"type": "error"}, done, stop),
            "failed terminal status": (done, stop | {"subtype": "error", "is_error": True}),
        }
        for reason, events in invalid.items():
            with self.subTest(reason=reason):
                self.assertFalse(dispatch_succeeded("cursor", output(*events)))

    def test_result_requires_the_whole_reply_and_success_status(self):
        body = {"type": "result", "subtype": "error", "is_error": True, "result": DONE}
        self.assertFalse(dispatch_succeeded("claude", io.StringIO(json.dumps(body))))
        self.assertFalse(dispatch_succeeded("pi", io.StringIO(DONE + "\n" + " " * 8192 + "still running")))
        self.assertFalse(dispatch_succeeded("custom", io.StringIO("Tool output: " + DONE)))

    def test_real_subprocess_command_contract_for_each_host(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            args = SimpleNamespace(board=4, name="worker", project=root)
            helper = root / "fake.py"
            helper.write_text("""import json, os, sys
from pathlib import Path
host, *args = sys.argv[1:]
assert Path.cwd().name == Path(__file__).parent.name
assert os.environ["SLATE_WATCH_NAME"] == "worker"
assert os.environ["SLATE_WATCH_BOARD"] == "4"
assert os.environ["SLATE_WATCH_URL"] == "http://localhost/mcp?name=worker"
if host == "codex":
    assert args[:2] == ["exec", "--output-last-message"]
    assert args[-1] == "-"
    prompt = sys.stdin.read()
    Path(args[2]).write_text("SLATE_DISPATCH_DONE", encoding="utf-8")
elif host == "claude":
    assert args == ["--print", "--output-format", "json"]
    prompt = sys.stdin.read()
elif host in ("cursor", "cursor-error"):
    assert args[:2] == ["--print", "--output-format"]
    prompt = args[-1]
    progress = "Checking the board."
    done = "SLATE_DISPATCH_DONE"
    result = {"type": "result", "subtype": "success", "is_error": False, "result": progress + done}
    if args[2] == "json":
        print(json.dumps(result))
    else:
        assert args[2] == "stream-json"
        def message(role, text):
            return {"type": role, "message": {"role": role, "content": [{"type": "text", "text": text}]}}
        tool = {"type": "tool_call", "call_id": "call-1", "tool_call": {"function": {"name": "whoami", "arguments": "{}"}}}
        for event in [
            {"type": "system", "subtype": "init"}, message("user", prompt), message("assistant", progress),
            dict(tool, subtype="started"), dict(tool, subtype="completed"), message("assistant", done), result,
        ]:
            print(json.dumps(dict(event, session_id="session-1")))
    if host == "cursor-error":
        sys.exit(1)
elif host == "opencode":
    assert args[:-1] == ["run", "--format", "json"]
    prompt = args[-1]
    print(json.dumps({"type": "text", "part": {"text": "SLATE_DISPATCH_DONE"}}))
    print(json.dumps({"type": "step_finish", "part": {"reason": "stop"}}))
elif host == "custom":
    assert args == ["literal; no shell"]
    prompt = sys.stdin.read()
    print("SLATE_DISPATCH_DONE")
else:
    assert args[:-1] == ["--print", "--mode", "text"]
    prompt = args[-1]
    print("SLATE_DISPATCH_DONE")
assert "whoami" in prompt and '"card_id": 5' in prompt and "DROP RAW PAYLOAD" not in prompt
if host == "claude":
    print(json.dumps({"type": "result", "subtype": "success", "is_error": False, "result": "SLATE_DISPATCH_DONE"}))
""", encoding="utf-8")
            for host in ("codex", "cursor", "claude", "opencode", "pi", "on-my-pi", "custom"):
                with self.subTest(host=host):
                    args.codex_legacy_receipt = host == "codex"
                    command = [sys.executable, str(helper), host]
                    if host == "custom":
                        command.append("literal; no shell")
                    dispatch_agent(
                        args, "http://localhost/mcp?name=worker", host, command,
                        [{**EVENT, "payload": "DROP RAW PAYLOAD"}], False,
                    )
                    if host == "cursor":
                        command[2] = "cursor-error"
                        with self.assertRaises(PendingDispatch):
                            dispatch_agent(args, "http://localhost/mcp?name=worker", host, command, [EVENT], False)

    def test_process_failure_keeps_batch_pending(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            args = SimpleNamespace(board=4, name="worker", project=root)
            state = WatchState(root / "watch.sqlite3")
            try:
                state.processed(41)
                command = [sys.executable, "-c", f'print("{DONE}"); raise SystemExit(1)']

                def dispatch(events, reconcile):
                    dispatch_agent(args, "http://localhost/mcp", "custom", command, events, reconcile)

                with self.assertRaises(PendingDispatch):
                    handle_batch(state, dispatch, updates(events=[EVENT]))
                self.assertEqual(state.status(), (1, 41, 42))
            finally:
                state.close()

    def test_codex_receipt_controls_acknowledgement_and_next_event(self):
        with (tempfile.TemporaryDirectory() as directory,
              closing(WatchState(Path(directory) / "watch.sqlite3")) as state):
            root = Path(directory)
            args = SimpleNamespace(board=4, name="worker", project=root)
            helper = root / "codex.py"
            helper.write_text("""import json, re, sys
from pathlib import Path

mode, *args = sys.argv[1:]
prompt = sys.stdin.read()
match = re.search(r'\\bdispatch_id: ([0-9a-f]{32})\\b', prompt)
dispatch_id = match.group(1) if match else None
result = Path(args[args.index('--output-last-message') + 1])
reply = {'dispatch_id': dispatch_id, 'outcome': 'processed'}
if mode == 'tool-only':
    print('SLATE_DISPATCH_DONE')
    result.write_text('Still working.')
elif mode == 'old-marker':
    result.write_text('SLATE_DISPATCH_DONE')
elif mode == 'missing':
    pass
elif mode == 'mismatch':
    result.write_text(json.dumps(reply | {'dispatch_id': 'another-batch'}))
elif mode == 'duplicate':
    result.write_text(json.dumps(reply)[:-1] + ',"outcome":"processed"}')
elif mode == 'nonzero':
    result.write_text(json.dumps(reply))
    sys.exit(9)
elif mode == 'unconfirmed':
    result.write_text(json.dumps(reply | {'outcome': 'unconfirmed'}))
else:
    schema = json.loads(Path(args[args.index('--output-schema') + 1]).read_text())
    assert schema['additionalProperties'] is False
    assert sorted(schema['required']) == ['dispatch_id', 'outcome']
    assert dispatch_id and '等待人工补验' in prompt
    result.write_text(json.dumps(reply | {'outcome': mode}))
""", encoding="utf-8")
            state.processed(41)
            mcp = Mock()
            mcp.tool.side_effect = [updates(events=[EVENT]), updates(43, events=[
                {"id": 43, "type": "comment.added", "card_id": 5},
            ])]

            def dispatch(events, reconcile):
                mode = "waiting_external" if events[0]["id"] == 42 else "processed"
                dispatch_agent(args, "http://localhost/mcp", "codex", [sys.executable, str(helper), mode], events, reconcile)

            watch_once(state, mcp, dispatch, 4, 5)
            self.assertEqual(state.status(), (1, 42, None))
            watch_once(state, mcp, dispatch, 4, 5)
            mcp.tool.assert_called_with("wait_for_updates", {"board_id": 4, "since": 42, "timeout_seconds": 5}, 15)
            self.assertEqual(state.status(), (1, 43, None))

            for mode, reason in {
                "missing": "receipt_missing", "mismatch": "receipt_dispatch_mismatch",
                "duplicate": "receipt_invalid", "nonzero": "host_exit (9)",
                "tool-only": "receipt_invalid", "old-marker": "receipt_invalid", "unconfirmed": "agent_unconfirmed",
            }.items():
                with self.subTest(mode=mode):
                    def failed_dispatch(events, reconcile):
                        dispatch_agent(args, "http://localhost/mcp", "codex", [sys.executable, str(helper), mode], events, reconcile)

                    with self.assertRaisesRegex(PendingDispatch, re.escape(reason)):
                        handle_batch(state, failed_dispatch, updates(44, events=[EVENT]), host="codex")
                    self.assertEqual(state.status(), (1, 43, 44))
                    self.assertEqual(state.dispatch_status(), ("codex", reason))
                    state.resolve("retry")


if __name__ == "__main__":
    unittest.main()
