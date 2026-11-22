"""Exercise the review integrator against disposable Git repositories."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("integrate.py")
KEY = "SLATE-42"
BRANCH = "slate/slate-42-reviewed"


class IntegrationTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix="slate-integrate-test-")
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.project = self.root / "project"
        self.project.mkdir()
        self.source = self.root / "project-slate-42"
        self.card_json = self.root / "card.json"
        self.git(self.project, "init", "-b", "dev")
        self.git(self.project, "config", "user.name", "Slate Test")
        self.git(self.project, "config", "user.email", "test@example.invalid")
        self.git(self.project, "config", "commit.gpgsign", "false")
        self.base = self.commit(self.project, "shared", "base")
        self.git(self.project, "worktree", "add", "-b", BRANCH, str(self.source), self.base)

    @staticmethod
    def git(path, *args):
        return subprocess.check_output(["git", "-C", str(path), *args], text=True, stderr=subprocess.STDOUT).strip()

    def commit(self, path, file, content):
        (path / file).write_text(content + "\n", encoding="utf-8")
        self.git(path, "add", file)
        self.git(path, "commit", "-m", content)
        return self.git(path, "rev-parse", "HEAD")

    def write_card(self, source, *, merged=None, note=None, approval=77):
        self.card_json.write_text(json.dumps({
            "id": 42, "key": KEY, "branch": BRANCH, "done": merged is not None,
            "claim_active": merged is None, "merged_commits": merged,
            "comments": [{"id": approval, "author_kind": "agent",
                          "body": note or f"review passed\nSlate review: {self.base}..{source}"}],
        }), encoding="utf-8")

    def run_script(self, action, *args, succeeds=True, approval=77, env=None):
        command = [sys.executable, str(SCRIPT), "--project", str(self.project)]
        if action != "pending":
            command += ["--card-id", "42", "--key", KEY, "--branch", BRANCH,
                        "--approval", str(approval), "--card-json", str(self.card_json)]
        result = subprocess.run([*command, action, *args], capture_output=True, text=True, env=env)
        if succeeds:
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads(result.stdout)
        self.assertEqual(result.returncode, 1, result.stderr)
        return result.stderr

    def apply(self, head, **kwargs):
        return self.run_script("apply", "--base", self.base, "--reviewed-head", head, **kwargs)

    def git_shim(self, body):
        bin_path = self.root / "bin"
        bin_path.mkdir()
        shim = bin_path / "git"
        shim.write_text(f"""#!{sys.executable}
import pathlib
import subprocess
import sys

args = sys.argv[1:]
git = {shutil.which('git')!r}
{body}
sys.exit(subprocess.call([git, *args]))
""", encoding="utf-8")
        shim.chmod(0o700)
        return {**os.environ, "PATH": str(bin_path) + os.pathsep + os.environ["PATH"]}

    def race_at_ref_update(self, action, *git_args):
        log = self.root / "race.log"
        env = self.git_shim(f"""
log = pathlib.Path({str(log)!r})
updates_ref = 'update-ref' in args and '--stdin' in args
old_publish = 'merge' in args and '--ff-only' in args
old_cleanup = 'update-ref' in args and '-d' in args and {f'refs/heads/{BRANCH}'!r} in args
if not log.exists() and (updates_ref or ({action!r} == 'apply' and old_publish)
                        or ({action!r} == 'cleanup' and old_cleanup)):
    raced = subprocess.run([git, '-C', {str(self.project)!r}, *{git_args!r}], capture_output=True)
    log.write_text(str(raced.returncode))
""")
        return env, log

    @unittest.skipIf(os.name == "nt", "executable Git shim requires a POSIX shebang")
    def test_concurrent_switch_cannot_update_another_branch(self):
        self.git(self.project, "branch", "unrelated")
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        env, race = self.race_at_ref_update("apply", "switch", "unrelated")

        receipt = self.apply(head, env=env)

        self.assertNotEqual(race.read_text(), "0")
        self.assertEqual(self.git(self.project, "rev-parse", "unrelated"), self.base)
        self.assertEqual(self.git(self.project, "rev-parse", "dev"), receipt["head"])
        self.assertEqual(self.git(self.project, "status", "--porcelain"), "")

    @unittest.skipIf(os.name == "nt", "executable Git shim requires a POSIX shebang")
    def test_publish_rechecks_head_after_the_reference_transaction_locks_it(self):
        self.git(self.project, "branch", "unrelated")
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        # Model a switch that has already finished its index phase and is updating HEAD.
        env, race = self.race_at_ref_update("apply", "symbolic-ref", "HEAD", "refs/heads/unrelated")

        self.assertIn("main branch changed during staging", self.apply(head, env=env, succeeds=False))

        self.assertEqual(race.read_text(), "0")
        self.assertEqual(self.git(self.project, "rev-parse", "unrelated"), self.base)
        self.assertEqual(self.git(self.project, "rev-parse", "dev"), self.base)
        self.assertEqual(self.git(self.project, "status", "--porcelain"), "")

    @unittest.skipIf(os.name == "nt", "executable Git shim requires a POSIX shebang")
    def test_publish_keeps_a_concurrently_advanced_target(self):
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        changed = self.git(self.project, "commit-tree", f"{self.base}^{{tree}}", "-p", self.base, "-m", "raced")
        env, race = self.race_at_ref_update("apply", "update-ref", "refs/heads/dev", changed, self.base)

        self.assertIn("reference transaction", self.apply(head, env=env, succeeds=False))

        self.assertEqual(race.read_text(), "0")
        self.assertEqual(self.git(self.project, "rev-parse", "dev"), changed)
        self.assertEqual(self.git(self.project, "status", "--porcelain"), "")

    @unittest.skipIf(os.name == "nt", "executable Git shim requires a POSIX shebang")
    def test_concurrent_checkout_cannot_lose_its_branch(self):
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        receipt = self.apply(head)
        self.write_card(head, merged=receipt["merged_commits"])
        racer = self.root / "racer"
        env, race = self.race_at_ref_update("cleanup", "worktree", "add", str(racer), BRANCH)

        self.assertTrue(self.run_script("cleanup", env=env)["cleaned"])

        self.assertNotEqual(race.read_text(), "0")
        self.assertFalse(racer.exists())
        self.assertFalse(self.source.exists())

    @unittest.skipIf(os.name == "nt", "executable Git shim requires a POSIX shebang")
    def test_cleanup_does_not_delete_a_concurrently_advanced_branch(self):
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        receipt = self.apply(head)
        self.write_card(head, merged=receipt["merged_commits"])
        changed = self.git(self.project, "commit-tree", f"{head}^{{tree}}", "-p", head, "-m", "raced")
        branch_ref = f"refs/heads/{BRANCH}"
        env, race = self.race_at_ref_update("cleanup", "update-ref", branch_ref, changed, head)

        self.assertIn("reference transaction", self.run_script("cleanup", env=env, succeeds=False))

        self.assertEqual(race.read_text(), "0")
        self.assertEqual(self.git(self.source, "rev-parse", "HEAD"), changed)
        self.assertTrue(self.source.exists())

    @unittest.skipIf(os.name == "nt", "executable Git shim requires a POSIX shebang")
    def test_refusal_after_tree_update_preserves_the_prepared_index(self):
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        env = self.git_shim("""
if 'update-ref' in args and '--stdin' in args:
    with subprocess.Popen([git, *args], stdin=subprocess.PIPE, text=True) as child:
        for line in sys.stdin:
            child.stdin.write('abort\\n' if line == 'commit\\n' else line)
            child.stdin.flush()
        child.stdin.close()
    sys.exit(1)
""")

        self.assertIn("Inspect them before retrying", self.apply(head, env=env, succeeds=False))

        self.assertEqual(self.git(self.project, "rev-parse", "dev"), self.base)
        self.assertEqual(self.git(self.project, "status", "--porcelain"), "A  one")
        self.assertTrue((self.project / "one").is_file())
        self.assertIn("worktree has changes", self.apply(head, succeeds=False))
        self.assertEqual(self.git(self.project, "status", "--porcelain"), "A  one")
        self.assertEqual(len(self.run_script("pending")), 1)

    @unittest.skipIf(os.name == "nt", "executable Git shim requires a POSIX shebang")
    def test_cleanup_resumes_after_branch_deleted_before_detaching(self):
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        receipt = self.apply(head)
        self.write_card(head, merged=receipt["merged_commits"])
        env = self.git_shim(f"""
marker = pathlib.Path({str(self.root / 'delete-started')!r})
if 'update-ref' in args and '--stdin' in args:
    if marker.exists():
        sys.exit(1)
    marker.touch()
""")

        self.assertIn("reference transaction", self.run_script("cleanup", env=env, succeeds=False))
        self.assertTrue(self.source.exists())
        self.assertEqual(self.git(self.project, "branch", "--list", BRANCH), "")
        self.assertTrue(self.run_script("cleanup")["cleaned"])
        self.assertFalse(self.source.exists())

    def test_applies_reviewed_commits_once_and_cleans_after_confirmation(self):
        self.commit(self.source, "one", "task one")
        head = self.commit(self.source, "two", "task two")
        before = self.commit(self.project, "main", "main advanced")
        self.write_card(head)
        applied = self.apply(head)

        ordered = self.git(self.project, "rev-list", "--reverse", f"{before}..HEAD").splitlines()
        self.assertEqual([sha[:6] for sha in ordered], applied["merged_commits"])
        self.assertEqual(len(ordered), 2)
        self.assertEqual(self.git(self.project, "rev-parse", "HEAD"), applied["head"])
        self.assertTrue((self.project / "one").is_file() and (self.project / "two").is_file())
        self.assertEqual(self.apply(head), applied)
        self.assertEqual(self.run_script("pending"), [applied])
        self.assertIn("card does not confirm", self.run_script("cleanup", succeeds=False))
        self.assertTrue(self.source.exists())

        self.write_card(head, merged=applied["merged_commits"])
        (self.source / "unsaved").write_text("retain me", encoding="utf-8")
        self.assertIn("worktree has changes", self.run_script("cleanup", succeeds=False))
        self.assertTrue((self.source / "unsaved").exists())
        (self.source / "unsaved").unlink()
        ignored = self.source / "ignored-secret"
        ignored.write_text("keep this too", encoding="utf-8")
        (self.project / ".git" / "info" / "exclude").write_text("ignored-secret\n", encoding="utf-8")
        self.assertIn("worktree has changes", self.run_script("cleanup", succeeds=False))
        self.assertTrue(ignored.exists())
        ignored.unlink()
        self.assertTrue(self.run_script("cleanup")["cleaned"])
        self.assertFalse(self.source.exists())
        self.assertEqual(self.git(self.project, "branch", "--list", BRANCH), "")
        self.assertEqual(self.run_script("pending"), [])
        self.assertTrue(self.run_script("cleanup")["cleaned"])

    def assert_cleanup_preserves_flagged_file(self, flag, name):
        head = self.commit(self.source, name, "reviewed content")
        self.write_card(head)
        receipt = self.apply(head)
        self.write_card(head, merged=receipt["merged_commits"])
        self.git(self.source, "update-index", flag, "--", name)
        index = Path(self.git(self.source, "rev-parse", "--path-format=absolute", "--git-path", "index"))
        local = self.source / name
        local.write_text("unique local data\n", encoding="utf-8")
        self.assertEqual(self.git(self.source, "status", "--porcelain"), "")
        saved_index = index.read_bytes()

        error = self.run_script("cleanup", succeeds=False)

        self.assertIn("index can hide local changes", error)
        self.assertIn(repr(name), error)
        self.assertEqual(local.read_text(encoding="utf-8"), "unique local data\n")
        self.assertEqual(index.read_bytes(), saved_index)
        self.assertEqual(self.git(self.source, "rev-parse", "HEAD"), head)
        self.assertEqual(self.git(self.project, "rev-parse", BRANCH), head)

    def test_cleanup_preserves_assume_unchanged_file(self):
        self.assert_cleanup_preserves_flagged_file("--assume-unchanged", "tracked")

    @unittest.skipIf(os.name == "nt", "Windows filenames cannot contain newlines")
    def test_cleanup_preserves_skip_worktree_file_with_newline(self):
        self.assert_cleanup_preserves_flagged_file("--skip-worktree", "first\nsecond")

    @unittest.skipIf(os.name == "nt", "test passes a raw non-UTF-8 path to Git")
    def test_cleanup_reads_non_utf8_index_path(self):
        head = self.commit(self.source, "tracked", "reviewed content")
        self.write_card(head)
        receipt = self.apply(head)
        self.write_card(head, merged=receipt["merged_commits"])
        name = b"unusual-\xff"
        blob = self.git(self.source, "rev-parse", "HEAD:tracked").encode("ascii")
        self.git(self.source, "update-index", "--add", "--cacheinfo", b"100644," + blob + b"," + name)
        self.git(self.source, "update-index", "--skip-worktree", "--", name)

        error = self.run_script("cleanup", succeeds=False)

        self.assertIn("index can hide local changes", error)
        self.assertIn(repr(os.fsdecode(name)), error)
        self.assertEqual(self.git(self.project, "rev-parse", BRANCH), head)

    def test_resumes_staged_receipt_only_for_the_same_approval(self):
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        receipt = self.apply(head)
        # Simulate the window after the staging receipt was written but before publishing.
        self.git(self.project, "reset", "--hard", self.base)
        self.write_card(head, note="review passed on another revision")
        self.assertIn("does not contain the approval", self.apply(head, succeeds=False))
        self.assertEqual(self.git(self.project, "rev-parse", "HEAD"), self.base)
        self.write_card(head)
        self.assertEqual(self.apply(head), receipt)
        self.assertEqual(self.git(self.project, "rev-parse", "HEAD"), receipt["head"])

    def test_preserves_main_and_task_snapshots_on_dirty_worktree_or_conflict(self):
        head = self.commit(self.source, "shared", "reviewed replacement")
        before = self.commit(self.project, "shared", "main replacement")
        self.write_card(head)
        note = self.project / "unsaved"
        note.write_text("keep this", encoding="utf-8")
        self.assertIn("worktree has changes", self.apply(head, succeeds=False))
        self.assertEqual(note.read_text(encoding="utf-8"), "keep this")
        note.unlink()

        self.assertIn("cherry-pick", self.apply(head, succeeds=False))
        self.assertEqual(self.git(self.project, "rev-parse", "HEAD"), before)
        self.assertEqual(self.git(self.source, "rev-parse", "HEAD"), head)
        self.assertEqual(self.git(self.project, "status", "--porcelain"), "")
        self.assertEqual(self.git(self.source, "status", "--porcelain"), "")
        self.assertEqual(self.run_script("pending"), [])
        self.assertEqual(self.git(self.project, "worktree", "list").count("\n"), 1)

    def test_publish_preserves_ignored_file_in_the_way(self):
        head = self.commit(self.source, "one", "task one")
        self.write_card(head)
        (self.project / ".git" / "info" / "exclude").write_text("one\n", encoding="utf-8")
        local = self.project / "one"
        local.write_text("retain ignored file", encoding="utf-8")

        self.assertIn("ignored local path", self.apply(head, succeeds=False))

        self.assertEqual(local.read_text(encoding="utf-8"), "retain ignored file")
        self.assertEqual(self.git(self.project, "rev-parse", "HEAD"), self.base)
        self.assertEqual(self.git(self.project, "status", "--porcelain"), "")
        local.unlink()
        self.assertEqual(self.apply(head)["merged_commits"], [self.git(self.project, "rev-parse", "HEAD")[:6]])


if __name__ == "__main__":
    unittest.main()
