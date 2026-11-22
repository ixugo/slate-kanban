#!/usr/bin/env python3
"""Integrate the exact commits approved for a local Slate task."""

import argparse
import contextlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


class IntegrationError(Exception):
    pass


def git_environment():
    # A host invoked from another checkout may export these variables.
    inherited = {"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX"}
    return {key: value for key, value in os.environ.items() if key not in inherited}


def git(path, *args, check=True, index=None, raw=False):
    env = git_environment()
    if index is not None:
        env["GIT_INDEX_FILE"] = str(index)
    result = subprocess.run(
        ["git", "-C", str(path), "--no-replace-objects", *args],
        capture_output=True, text=not raw, env=env,
    )
    if result.returncode and check:
        detail = result.stderr.strip() or result.stdout.strip()
        if raw:
            detail = os.fsdecode(detail)
        raise IntegrationError(f"git {args[0]}: {detail}")
    if result.returncode != 0:
        return None
    return result.stdout if raw else result.stdout.rstrip("\n")


@contextlib.contextmanager
def lock_index(path):
    index = Path(git(path, "rev-parse", "--path-format=absolute", "--git-path", "index"))
    lock_path = index.with_name(index.name + ".lock")
    try:
        lock_path.touch(exist_ok=False)
    except FileExistsError as error:
        raise IntegrationError(f"another Git operation holds the index: {path}") from error
    try:
        yield index
    finally:
        # Removing a linked worktree also removes its index lock.
        lock_path.unlink(missing_ok=True)


@contextlib.contextmanager
def prepared_ref_update(path, state, command):
    # Git owns the reference locks and discards an uncommitted transaction on EOF.
    with tempfile.TemporaryFile(mode="w+t") as errors:
        with subprocess.Popen(
            ["git", "-C", str(path), "--no-replace-objects", "-c", f"core.hooksPath={state / 'no-hooks'}",
             "update-ref", "--stdin"],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors, text=True, env=git_environment(),
        ) as process:
            def execute(message, *expected):
                try:
                    process.stdin.write(message)
                    process.stdin.flush()
                    if all(process.stdout.readline().strip() == reply for reply in expected):
                        return
                except BrokenPipeError:
                    pass
                process.stdin.close()
                process.wait()
                errors.seek(0)
                raise IntegrationError(f"Git reference transaction failed: {errors.read().strip()}")

            try:
                execute(f"start\n{command}\nprepare\n", "start: ok", "prepare: ok")
                yield
                execute("commit\n", "commit: ok")
            finally:
                if not process.stdin.closed:
                    process.stdin.close()


def worktrees(project):
    output = git(project, "worktree", "list", "--porcelain", "-z")
    return [dict(field.split(" ", 1) if " " in field else (field, "") for field in row.split("\0"))
            for row in output.rstrip("\0").split("\0\0")]


def require_clean(path, include_ignored=False):
    args = ["status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none"]
    if include_ignored:
        args.append("--ignored=matching")
    if git(path, *args):
        raise IntegrationError(f"worktree has changes: {path}")
    operations = ("CHERRY_PICK_HEAD", "MERGE_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer", "BISECT_START")
    if any(Path(git(path, "rev-parse", "--path-format=absolute", "--git-path", name)).exists() for name in operations):
        raise IntegrationError(f"a Git operation is in progress: {path}")


def require_visible_index_entries(path):
    # Git emits lowercase tags for assume-unchanged and S for skip-worktree.
    entries = git(path, "ls-files", "-v", "-z", raw=True)
    for entry in entries.split(b"\0"):
        tag = entry[:1]
        if tag == b"S" or tag.islower():
            name = os.fsdecode(entry[2:])
            raise IntegrationError(f"task worktree index can hide local changes for {name!r}")


def is_ancestor(project, before, after):
    return git(project, "merge-base", "--is-ancestor", before, after, check=False) is not None


@contextlib.contextmanager
def repository_lock(path):
    with path.open("a+b") as handle:
        if os.name == "nt":
            import msvcrt
            handle.seek(0)
            try:
                msvcrt.locking(handle.fileno(), msvcrt.LK_NBLCK, 1)
            except OSError as error:
                raise IntegrationError("another Slate integration is running") from error
        else:
            import fcntl
            try:
                fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError as error:
                raise IntegrationError("another Slate integration is running") from error
        # Closing the handle also releases the lock if the integration is interrupted.
        yield


def write_receipt(path, data):
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent, delete=False, encoding="utf-8") as handle:
        temporary = Path(handle.name)
        try:
            json.dump(data, handle)
            handle.flush()
            os.fsync(handle.fileno())
        except BaseException:
            temporary.unlink()
            raise
    try:
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def source_worktree(project, branch_ref, task_path):
    matches = [row for row in worktrees(project) if row.get("branch") == branch_ref]
    if not matches:
        if task_path.exists():
            raise IntegrationError("expected task worktree path is occupied")
        return None
    if len(matches) != 1 or Path(matches[0]["worktree"]).resolve() != task_path:
        raise IntegrationError("branch is checked out outside the task's dedicated worktree")
    if "locked" in matches[0] or "prunable" in matches[0]:
        raise IntegrationError("task worktree is locked or missing")
    return matches[0]


def card_evidence(args, expected, base, head):
    card = json.loads(args.card_json.read_text(encoding="utf-8"))
    fields = {"id": args.card_id, "key": args.key, "branch": args.branch, **expected}
    if any(card.get(k) != v for k, v in fields.items()):
        raise IntegrationError("card does not confirm this review and task state")
    for comment in card.get("comments", []):
        if (comment.get("id") == args.approval and comment.get("author_kind") == "agent"
                and f"Slate review: {base}..{head}" in comment.get("body", "").splitlines()):
            return card
    raise IntegrationError("card does not contain the approval for these exact commits")


def apply(args, project, state, receipt_path, task_path, branch_ref):
    card_evidence(args, {"done": False, "claim_active": True}, args.base, args.reviewed_head)
    require_clean(project)
    target_ref = git(project, "symbolic-ref", "-q", "HEAD")
    before = git(project, "rev-parse", "--verify", target_ref)
    if target_ref == branch_ref:
        raise IntegrationError("task branch is checked out in the main worktree")
    source = source_worktree(project, branch_ref, task_path)
    if source is None or source["HEAD"] != args.reviewed_head:
        raise IntegrationError("task worktree no longer points at the reviewed commit")
    require_clean(task_path)
    requested = {"branch": args.branch, "base": args.base, "reviewed_head": args.reviewed_head,
                 "approval": args.approval, "card_id": args.card_id, "key": args.key, "target_ref": target_ref}

    if receipt_path.exists():
        receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
        if any(receipt.get(k) != v for k, v in requested.items()):
            raise IntegrationError("saved integration belongs to different review evidence")
        if is_ancestor(project, receipt["head"], before):
            return receipt
        if receipt["before"] != before:
            raise IntegrationError("main branch changed after staging; inspect the saved integration")
    else:
        if not is_ancestor(project, args.base, before) or not is_ancestor(project, args.base, args.reviewed_head):
            raise IntegrationError("recorded task base must be an ancestor of both branches")
        history = f"{args.base}..{args.reviewed_head}"
        if git(project, "rev-list", "--merges", history):
            raise IntegrationError("task contains merge commits; a linear reviewed range is required")
        commits = git(project, "rev-list", "--reverse", history).splitlines()
        if not commits:
            raise IntegrationError("reviewed task has no commits")
        receipt = stage(project, state, requested, before, commits, receipt_path)

    publish(project, state, receipt, branch_ref)
    return receipt


def publish(project, state, receipt, branch_ref):
    target_ref, before, head = (receipt[key] for key in ("target_ref", "before", "head"))
    with lock_index(project) as index:
        synchronized = False
        try:
            # Updating HEAD locks both the symbolic reference and its referent. Verify the
            # exact symbolic target after prepare, while a concurrent switch cannot change it.
            command = f"update HEAD {head} {before}\nverify {branch_ref} {receipt['reviewed_head']}"
            with prepared_ref_update(project, state, command):
                if git(project, "symbolic-ref", "-q", "HEAD", check=False) != target_ref:
                    raise IntegrationError("main branch changed during staging")
                require_clean(project)
                protect_ignored_paths(project, before, head)
                with tempfile.TemporaryDirectory(prefix="slate-index-", dir=index.parent) as temporary:
                    candidate = Path(temporary) / "index"
                    shutil.copyfile(index, candidate)
                    synchronized = True
                    # Use Git's two-tree safety checks for tracked and untracked files.
                    git(project, "read-tree", "-m", "-u", before, head, index=candidate)
                    candidate.replace(index)
        except (IntegrationError, OSError) as error:
            if synchronized:
                raise IntegrationError("Publishing may have changed the main worktree or index. "
                                       f"Inspect them before retrying. {error}") from error
            raise

    target = git(project, "rev-parse", "--verify", target_ref)
    if not is_ancestor(project, head, target):
        raise IntegrationError("target branch does not contain the staged integration")


def protect_ignored_paths(project, before, head):
    # Unlike merge --no-overwrite-ignore, read-tree may replace ignored paths.
    ignored = git(project, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
    if not ignored:
        return
    changed = git(project, "diff", "--name-only", "--no-renames", "-z", before, head, "--")
    ignore_case = git(project, "config", "--bool", "core.ignoreCase", check=False) == "true"
    changed_paths = changed.rstrip("\0").split("\0")
    if ignore_case:
        changed_paths = [path.casefold() for path in changed_paths]
    for path in ignored.rstrip("\0").split("\0"):
        candidate = path.rstrip("/")
        if ignore_case:
            candidate = candidate.casefold()
        if any(candidate == changed or candidate.startswith(changed + "/")
               or changed.startswith(candidate + "/") for changed in changed_paths):
            raise IntegrationError(f"ignored local path overlaps the integration: {path}")


def stage(project, state, requested, before, commits, receipt_path):
    with tempfile.TemporaryDirectory(prefix="stage-", dir=state) as parent:
        stage_path = Path(parent) / "worktree"
        git(project, "-c", f"core.hooksPath={state / 'no-hooks'}",
            "worktree", "add", "--detach", "--quiet", str(stage_path), before)
        try:
            git(stage_path, "-c", f"core.hooksPath={state / 'no-hooks'}", "-c", "rerere.enabled=false",
                "cherry-pick", "-x", *commits)
            head = git(stage_path, "rev-parse", "HEAD")
            hashes = git(stage_path, "rev-list", "--reverse", f"{before}..{head}").splitlines()
            receipt = {**requested, "before": before, "head": head,
                       "merged_commits": [value[:6] for value in hashes]}
            # Keep staged objects reachable across interruption before the main branch is updated.
            ref = f"refs/slate/integrations/{requested['card_id']}/{requested['approval']}"
            git(project, "update-ref", ref, head)
            receipt["staged_ref"] = ref
            write_receipt(receipt_path, receipt)
            return receipt
        finally:
            # Only this process's disposable staging worktree can be removed with force.
            git(project, "worktree", "remove", "--force", str(stage_path))


def cleanup(args, project, receipt_path, task_path, branch_ref):
    if not receipt_path.exists():
        raise IntegrationError("saved integration for the approved review is missing")
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    if receipt["key"] != args.key or receipt["branch"] != args.branch:
        raise IntegrationError("saved integration belongs to a different task branch")
    card_evidence(args, {"done": True, "merged_commits": receipt["merged_commits"]},
                  receipt["base"], receipt["reviewed_head"])
    integrated_tip(project, receipt)

    reviewed = receipt["reviewed_head"]
    source = cleanup_worktree(project, branch_ref, task_path)
    tip = git(project, "rev-parse", "--verify", branch_ref, check=False)
    if tip is not None and tip != reviewed:
        raise IntegrationError("task branch has changed since review")
    if tip is not None and (source is None or source.get("branch") != branch_ref):
        raise IntegrationError("task branch has lost its dedicated checkout. Preserve it for inspection.")
    if source is not None:
        identity = worktree_identity(task_path)
        with lock_index(task_path):
            require_visible_index_entries(task_path)
            if "cleanup_worktree" not in receipt:
                if tip is None or source["HEAD"] != reviewed or "locked" in source:
                    raise IntegrationError("task worktree is locked or has changed since review")
                require_clean(task_path, include_ignored=True)
                receipt["cleanup_worktree"] = identity
                write_receipt(receipt_path, receipt)
            if receipt["cleanup_worktree"] != identity:
                raise IntegrationError("task worktree is no longer the checkout recorded for cleanup")
            prepare_worktree_removal(project, receipt, receipt_path.parent, task_path, branch_ref, source)
            require_clean(task_path, include_ignored=True)
            if worktree_identity(task_path) != identity:
                raise IntegrationError("task worktree changed before removal")
            git(project, "worktree", "remove", str(task_path))
    if (cleanup_worktree(project, branch_ref, task_path) is not None
            or git(project, "rev-parse", "--verify", branch_ref, check=False) is not None):
        raise IntegrationError("task branch or worktree is still present after cleanup")
    if git(project, "rev-parse", "--verify", receipt["staged_ref"], check=False) is not None:
        target = integrated_tip(project, receipt)
        command = (f"verify {receipt['target_ref']} {target}\n"
                   f"delete {receipt['staged_ref']} {receipt['head']}")
        with prepared_ref_update(project, receipt_path.parent, command):
            pass
    write_receipt(receipt_path, {**receipt, "cleaned": True})
    return {"cleaned": True, "branch": args.branch}


def integrated_tip(project, receipt):
    target = git(project, "rev-parse", "--verify", receipt["target_ref"])
    if not is_ancestor(project, receipt["head"], target):
        raise IntegrationError("recorded target branch no longer contains the integrated commits")
    return target


def cleanup_worktree(project, branch_ref, task_path):
    task = None
    for row in worktrees(project):
        path = Path(row["worktree"]).resolve()
        if row.get("branch") == branch_ref and path != task_path:
            raise IntegrationError("task branch is checked out in another worktree")
        if path == task_path:
            if (row.get("branch") != branch_ref and "detached" not in row) or "prunable" in row:
                raise IntegrationError("task path is no longer a valid task worktree")
            task = row
    if task is None and task_path.exists():
        raise IntegrationError("expected task worktree path is occupied")
    return task


def worktree_identity(path):
    directory = Path(git(path, "rev-parse", "--absolute-git-dir"))
    identity = directory.stat()
    return [str(directory), identity.st_dev, identity.st_ino]


def prepare_worktree_removal(project, receipt, state, task_path, branch_ref, source):
    # Keep the branch occupied in its dedicated checkout until Git deletes the exact OID.
    # A normal switch or worktree add cannot take a branch checked out in another worktree.
    reason = f"Slate cleanup for card {receipt['card_id']} approval {receipt['approval']}"
    if "locked" not in source:
        git(project, "worktree", "lock", "--reason", reason, str(task_path))
    elif source["locked"] != reason:
        raise IntegrationError("task worktree is locked by another operation")
    reviewed = receipt["reviewed_head"]
    try:
        tip = git(project, "rev-parse", "--verify", branch_ref, check=False)
        if tip is not None:
            target = integrated_tip(project, receipt)
            command = f"verify {receipt['target_ref']} {target}\ndelete {branch_ref} {reviewed}"
            with prepared_ref_update(task_path, state, command):
                current = cleanup_worktree(project, branch_ref, task_path)
                if current is None or current.get("branch") != branch_ref:
                    raise IntegrationError("task worktree no longer occupies the reviewed branch")
                require_clean(task_path, include_ignored=True)
        if git(task_path, "symbolic-ref", "-q", "HEAD", check=False) == branch_ref:
            # Restore a detached HEAD at the same commit without changing any files or index.
            command = f"option no-deref\nupdate HEAD {reviewed}"
            with prepared_ref_update(task_path, state, command):
                if (git(task_path, "symbolic-ref", "-q", "HEAD", check=False) != branch_ref
                        or git(project, "rev-parse", "--verify", branch_ref, check=False) is not None):
                    raise IntegrationError("task branch changed during cleanup")
        if (git(task_path, "symbolic-ref", "-q", "HEAD", check=False) is not None
                or git(task_path, "rev-parse", "HEAD") != reviewed):
            raise IntegrationError("task worktree is no longer at the reviewed commit")
    finally:
        git(project, "worktree", "unlock", str(task_path))


def run(args):
    project = args.project.resolve(strict=True)
    main = Path(worktrees(project)[0]["worktree"]).resolve()
    if main != project or git(project, "rev-parse", "--is-bare-repository") != "false":
        raise IntegrationError("project must be the main Git worktree root")
    state = Path(git(project, "rev-parse", "--path-format=absolute", "--git-common-dir")) / "slate-integrations"
    if args.action == "pending":
        receipts = (json.loads(path.read_text(encoding="utf-8")) for path in sorted(state.glob("*.json")))
        return [receipt for receipt in receipts if not receipt.get("cleaned")]
    if not all((args.card_id, args.approval, args.key, args.branch, args.card_json)):
        raise IntegrationError("card ID, key, branch, approval and card JSON are required")
    if not re.fullmatch(r"[A-Z][A-Z0-9]+-[1-9][0-9]*", args.key):
        raise IntegrationError("expected a Slate card key")
    if args.card_id <= 0 or args.approval <= 0:
        raise IntegrationError("card and approval IDs must be positive")
    if not re.fullmatch(rf"slate/{args.key.lower()}-[a-z0-9][a-z0-9-]*", args.branch):
        raise IntegrationError("expected a card-specific Slate branch")
    state.mkdir(exist_ok=True)
    task_path = project.with_name(f"{project.name}-{args.key.lower()}")
    if task_path.is_symlink():
        raise IntegrationError("expected task worktree may not be a symlink")
    branch_ref = f"refs/heads/{args.branch}"
    receipt_path = state / f"{args.card_id}-{args.approval}.json"
    with repository_lock(state / "lock"):
        if args.action == "cleanup":
            return cleanup(args, project, receipt_path, task_path, branch_ref)
        for sha in (args.base, args.reviewed_head):
            if re.fullmatch(r"(?:[0-9a-f]{40}|[0-9a-f]{64})", sha) is None:
                raise IntegrationError("base and reviewed head must be complete Git hashes")
        return apply(args, project, state, receipt_path, task_path, branch_ref)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--project", type=Path, required=True)
    parser.add_argument("--card-id", type=int)
    parser.add_argument("--key")
    parser.add_argument("--branch")
    parser.add_argument("--approval", type=int,
                        help="ID of the saved agent approval comment for this exact reviewed head")
    parser.add_argument("--card-json", type=Path,
                        help="fresh structured get_card output")
    actions = parser.add_subparsers(dest="action", required=True)
    apply_parser = actions.add_parser("apply")
    apply_parser.add_argument("--base", required=True)
    apply_parser.add_argument("--reviewed-head", required=True)
    actions.add_parser("cleanup")
    actions.add_parser("pending")
    try:
        print(json.dumps(run(parser.parse_args())))
    except (IntegrationError, OSError, ValueError) as error:
        print(f"Slate integration: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
