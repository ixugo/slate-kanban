#!/usr/bin/env python3
"""Integrate the exact commits approved for a local Slate task."""

import argparse
import contextlib
import ctypes
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
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
    require_idle_git(path)


def require_idle_git(path):
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


def card_evidence(args, project, expected, base, head, receipt=None):
    card = json.loads(args.card_json.read_text(encoding="utf-8"))
    fields = {"id": args.card_id, "key": args.key, "branch": args.branch, **expected}
    if any(card.get(k) != v for k, v in fields.items()):
        raise IntegrationError("card does not confirm this review and task state")
    full_line = f"Slate review: {base}..{head}"
    short_line = f"Slate review: {base[:6]}..{head[:6]}"
    for comment in card.get("comments", []):
        if comment.get("id") != args.approval or comment.get("author_kind") != "agent":
            continue
        lines = comment.get("body", "").splitlines()
        if receipt is not None:
            # A receipt already binds this line to exact OIDs. New Git objects
            # may make a prefix ambiguous, and cleanup can remove the source.
            recorded_line = receipt.get("approval_line", full_line)
            if recorded_line in (full_line, short_line) and recorded_line in lines:
                return recorded_line
        elif full_line in lines:
            return full_line
        elif short_line in lines:
            for sha in (base, head):
                prefix = sha[:6]
                matches = git(project, "rev-parse", f"--disambiguate={prefix}").splitlines()
                if matches != [sha] or git(project, "cat-file", "-t", sha) != "commit":
                    raise IntegrationError(f"approval prefix {prefix} does not uniquely identify the reviewed commit")
            return short_line
        break
    raise IntegrationError("card does not contain the approval for these exact commits")


def apply(args, project, state, receipt_path, task_path, branch_ref):
    receipt = json.loads(receipt_path.read_text(encoding="utf-8")) if receipt_path.exists() else None
    approval_line = card_evidence(args, project, {"done": False, "claim_active": True},
                                  args.base, args.reviewed_head, receipt)
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

    if receipt is not None:
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
        receipt = stage(project, state, {**requested, "approval_line": approval_line},
                        before, commits, receipt_path)

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
    receipt = cleanup_receipt(args, project, receipt_path)
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
    receipt.pop("cleanup_error", None)
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


def cleanup_receipt(args, project, path):
    receipt = json.loads(path.read_text(encoding="utf-8"))
    if any(receipt[key] != getattr(args, key) for key in ("card_id", "approval", "key", "branch")):
        raise IntegrationError("saved integration belongs to a different task")
    card_evidence(args, project, {"done": True, "merged_commits": receipt["merged_commits"]},
                  receipt["base"], receipt["reviewed_head"], receipt)
    return receipt


def archive_destination(project, args):
    return project.with_name(f"{project.name}-slate-recovery") / f"{args.key.lower()}-{args.approval}"


def path_identity(path):
    info = path.lstat()
    return [info.st_dev, info.st_ino]


def local_snapshot(task):
    result = git(task, "status", "--porcelain=v1", "-z", "--untracked-files=all",
                 "--ignored=matching", "--ignore-submodules=none", "--no-renames", raw=True)
    snapshot = {}

    def fingerprint(info):
        return [info.st_mode, info.st_dev, info.st_ino, info.st_mtime_ns,
                info.st_ctime_ns, info.st_size]

    def visit(relative):
        name = relative.as_posix()
        if name in snapshot:
            return
        path = task / relative
        try:
            info = path.lstat()
        except FileNotFoundError:
            snapshot[name] = []
            return

        record = fingerprint(info)
        snapshot[name] = record
        if stat.S_ISLNK(info.st_mode):
            record.append(os.readlink(path))
        elif stat.S_ISREG(info.st_mode):
            flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_BINARY", 0)
            with os.fdopen(os.open(path, flags), "rb") as file:
                if fingerprint(os.fstat(file.fileno())) != record:
                    raise IntegrationError("a local file changed while preparing the preview")
                content = hashlib.sha256()
                for chunk in iter(lambda: file.read(1024 * 1024), b""):
                    content.update(chunk)
                record.append(content.hexdigest())
        elif stat.S_ISDIR(info.st_mode) and not getattr(info, "st_file_attributes", 0) & stat.FILE_ATTRIBUTE_REPARSE_POINT:
            with os.scandir(path) as children:
                names = sorted(child.name for child in children)
            for child in names:
                visit(relative / child)
        if fingerprint(path.lstat()) != record[:6]:
            raise IntegrationError("a local file changed while preparing the preview")

    for item in result.split(b"\0"):
        if item:
            visit(Path(os.fsdecode(item[3:]).rstrip("/")))
    return snapshot


def archive_preview(project, task, branch_ref, destination, receipt, index):
    source = cleanup_worktree(project, branch_ref, task)
    if (source is None or source.get("branch") != branch_ref or "locked" in source
            or source["HEAD"] != receipt["reviewed_head"]):
        raise IntegrationError("task worktree is locked or has changed since review")
    require_idle_git(task)
    require_visible_index_entries(task)
    snapshot = local_snapshot(task)
    if not snapshot:
        raise IntegrationError("no local files to archive")

    identity = worktree_identity(task)
    token = hashlib.sha256(json.dumps(
        [str(destination), identity, receipt["reviewed_head"], snapshot,
         hashlib.sha256(index.read_bytes()).hexdigest()],
    ).encode()).hexdigest()
    return {"status": "blocked", "files": list(snapshot), "destination": str(destination), "preview_token": token}


def check_private_directory(path):
    if path.is_symlink() or not path.is_dir():
        raise IntegrationError(f"recovery path is not a private directory: {path}")
    if os.name != "nt":
        info = path.stat()
        if info.st_uid != os.getuid() or info.st_mode & 0o077:
            raise IntegrationError(f"recovery path must be private to the current user: {path}")


def path_present(path):
    return path.exists() or path.is_symlink()


def rename_without_replacement(source, destination):
    # POSIX rename can replace an empty directory. A directory created by
    # another process between validation and publication must survive.
    if os.name == "nt":
        source.rename(destination)
        return
    libc = ctypes.CDLL(None, use_errno=True)
    old, new = os.fsencode(source), os.fsencode(destination)
    try:
        if sys.platform == "darwin":
            result = libc.renamex_np(old, new, 0x00000004)  # RENAME_EXCL
        elif sys.platform.startswith("linux"):
            result = libc.renameat2(-100, old, -100, new, 1)  # AT_FDCWD, RENAME_NOREPLACE
        else:
            raise IntegrationError("exclusive directory moves are unavailable on this system")
    except AttributeError as error:
        raise IntegrationError("exclusive directory moves are unavailable on this system") from error
    if result:
        code = ctypes.get_errno()
        raise OSError(code, os.strerror(code), str(destination))


def recovery_destination(project, args, journal):
    base = archive_destination(project, args)
    if journal is None:
        return base
    destination = Path(journal["destination"])
    if (destination.parent != base.parent
            or not re.fullmatch(re.escape(base.name) + r"(?:-[1-9][0-9]*)?", destination.name)):
        raise IntegrationError("saved recovery destination does not belong to this approval")
    return destination


def preview_local_archive(project, task, branch_ref, args, receipt, index):
    journal = receipt.get("cleanup_archive")
    destination = recovery_destination(project, args, journal)
    preview = archive_preview(project, task, branch_ref, destination, receipt, index)
    if journal and preview["preview_token"] != journal["preview_token"]:
        # A newer confirmation can use a new path. Keep the previous
        # preparation and any files the user may have added there.
        base = archive_destination(project, args)
        revision = 2
        while True:
            destination = base.with_name(f"{base.name}-{revision}")
            if str(destination) != journal["destination"] and not path_present(destination):
                break
            revision += 1
        preview = archive_preview(project, task, branch_ref, destination, receipt, index)
    return preview


def prepare_archive(project, destination, task, index, receipt, receipt_path, branch_ref, preview):
    destination.parent.mkdir(mode=0o700, exist_ok=True)
    check_private_directory(destination.parent)
    if path_present(destination):
        raise IntegrationError(f"recovery path is already occupied: {destination}")
    setup = Path(tempfile.mkdtemp(prefix=".slate-clean-", dir=destination.parent))
    setup_identity = path_identity(setup)
    saved = False
    try:
        replacement = setup / "replacement"
        replacement.mkdir()
        clean_index = setup / "clean-index"
        git(project, "read-tree", receipt["reviewed_head"], index=clean_index)
        git(project, "checkout-index", "--all", f"--prefix={replacement}{os.sep}", index=clean_index)
        shutil.copyfile(task / ".git", replacement / ".git")
        (setup / "staged.patch").write_bytes(git(task, "diff", "--cached", "--binary", "--no-ext-diff", raw=True))
        (setup / "unstaged.patch").write_bytes(git(task, "diff", "--binary", "--no-ext-diff", raw=True))
        shutil.copyfile(index, setup / "original-index")
        if archive_preview(project, task, branch_ref, destination, receipt, index) != preview:
            raise IntegrationError("archive confirmation is stale; review the current files")
        journal = {**preview, "source": path_identity(task), "replacement": path_identity(replacement),
                   "worktree": worktree_identity(task), "prepared": setup.name,
                   "prepared_identity": setup_identity}
        receipt["cleanup_archive"] = journal
        write_receipt(receipt_path, receipt)
        saved = True
        return journal
    finally:
        if not saved and path_present(setup) and path_identity(setup) == setup_identity:
            shutil.rmtree(setup)


def publish_archive(destination, journal):
    check_private_directory(destination.parent)
    if "prepared" not in journal:
        check_private_directory(destination)  # Resume a receipt from the original archive format.
        return
    name = journal["prepared"]
    if not re.fullmatch(r"\.slate-clean-[a-z0-9_]+", name):
        raise IntegrationError("invalid recovery preparation name")
    if not path_present(destination):
        setup = destination.parent / name
        check_private_directory(setup)
        if path_identity(setup) != journal["prepared_identity"]:
            raise IntegrationError("prepared recovery directory identity has changed")
        rename_without_replacement(setup, destination)
    check_private_directory(destination)
    if path_identity(destination) != journal["prepared_identity"]:
        raise IntegrationError(f"recovery path is already occupied: {destination}")


def archive_paths(project, destination, task, index, receipt, branch_ref, journal):
    files = destination / "files"
    replacement = destination / "replacement"
    if not path_present(files):
        if path_identity(task) != journal["source"] or worktree_identity(task) != journal["worktree"]:
            raise IntegrationError("original worktree has changed since confirmation")
        if archive_preview(project, task, branch_ref, destination, receipt, index)["preview_token"] != journal["preview_token"]:
            raise IntegrationError("archive confirmation is stale; review the current files")
        rename_without_replacement(task, files)
    if files.is_symlink() or path_identity(files) != journal["source"]:
        raise IntegrationError("archived worktree identity has changed")
    if not task.exists():
        if replacement.is_symlink() or path_identity(replacement) != journal["replacement"]:
            raise IntegrationError("replacement worktree identity has changed")
        rename_without_replacement(replacement, task)
    if task.is_symlink() or path_identity(task) != journal["replacement"]:
        raise IntegrationError("task worktree path has been occupied during recovery")
    if worktree_identity(task) != journal["worktree"]:
        raise IntegrationError("task worktree registration has changed")
    (files / ".git").unlink(missing_ok=True)
    # Replace the index under the original worktree's Git index lock.
    backup_index = destination / "clean-index"
    if backup_index.exists():
        with tempfile.NamedTemporaryFile(dir=index.parent, delete=False) as handle:
            temporary = Path(handle.name)
            handle.write(backup_index.read_bytes())
            handle.flush()
            os.fsync(handle.fileno())
        try:
            temporary.replace(index)
            backup_index.unlink()
        finally:
            temporary.unlink(missing_ok=True)
    require_clean(task, include_ignored=True)


def cleanup_status(receipt):
    journal = receipt.get("cleanup_archive")
    result = {"status": "cleaned" if receipt.get("cleaned") else "blocked", "reason": receipt.get("cleanup_error", "")}
    if journal:
        result.update(destination=journal["destination"], files=journal["files"],
                      preview_token=journal["preview_token"])
    if not receipt.get("cleaned") and not result["reason"]:
        result["status"] = "pending"
    return result


def cleanup_local(args, project, state, task, branch_ref):
    card = json.loads(args.card_json.read_text(encoding="utf-8"))
    candidates = []
    for path in sorted(state.glob(f"{args.card_id}-*.json")):
        receipt = json.loads(path.read_text(encoding="utf-8"))
        if receipt["branch"] == args.branch and receipt["merged_commits"] == card.get("merged_commits"):
            candidates.append(path)
    if not candidates:
        return {"status": "unknown"}
    if len(candidates) != 1:
        raise IntegrationError("multiple local receipts match this merge")
    receipt_path = candidates[0]
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    if args.approval is not None and args.approval != receipt["approval"]:
        raise IntegrationError("saved integration belongs to a different approval")
    args.approval = receipt["approval"]
    receipt = cleanup_receipt(args, project, receipt_path)
    action = args.action
    if receipt.get("cleaned"):
        return cleanup_status(receipt)
    if action == "cleanup-status":
        result = cleanup_status(receipt)
        if result["status"] == "pending":
            try:
                source = cleanup_worktree(project, branch_ref, task)
                if source is not None:
                    if "locked" in source:
                        raise IntegrationError("task worktree is locked")
                    index_path = Path(git(task, "rev-parse", "--path-format=absolute", "--git-path", "index"))
                    if index_path.with_name(index_path.name + ".lock").exists():
                        raise IntegrationError(f"another Git operation holds the index: {task}")
                    require_visible_index_entries(task)
                    require_clean(task, include_ignored=True)
            except (IntegrationError, OSError) as error:
                result = {"status": "blocked", "reason": str(error)}
        return result
    try:
        journal = receipt.get("cleanup_archive")
        destination = recovery_destination(project, args, journal)
        files = destination / "files"
        archive_started = bool(journal and path_present(files))
        if archive_started and (files.is_symlink() or path_identity(files) != journal["source"]):
            raise IntegrationError("archived worktree identity has changed")
        integrated_tip(project, receipt)
        if action == "cleanup-retry":
            cleanup(args, project, receipt_path, task, branch_ref)
            return cleanup_status(json.loads(receipt_path.read_text(encoding="utf-8")))
        if action == "cleanup-preview" and archive_started:
            publish_archive(destination, journal)
            return {key: journal[key] for key in ("status", "files", "destination", "preview_token")}
        if not archive_started:
            with lock_index(task) as index:
                preview = preview_local_archive(project, task, branch_ref, args, receipt, index)
                if action == "cleanup-preview":
                    return preview
                if preview["preview_token"] != args.preview_token:
                    raise IntegrationError("archive confirmation is stale; review the current files")
                destination = Path(preview["destination"])
                if not journal or journal["preview_token"] != args.preview_token:
                    journal = prepare_archive(project, destination, task, index, receipt, receipt_path,
                                              branch_ref, preview)
        elif journal["preview_token"] != args.preview_token:
            raise IntegrationError("archive confirmation does not match the recorded preview")
        # Publishing the prepared directory does not touch the checkout. No
        # index lock is needed here, so process exit cannot strand a lock.
        publish_archive(destination, journal)
        if not archive_started or not task.exists() or (destination / "clean-index").exists():
            lock_target = task if task.exists() else destination / "files"
            with lock_index(lock_target) as index:
                integrated_tip(project, receipt)
                archive_paths(project, destination, task, index, receipt, branch_ref, journal)
        cleanup(args, project, receipt_path, task, branch_ref)
        return cleanup_status(json.loads(receipt_path.read_text(encoding="utf-8")))
    except (IntegrationError, OSError) as error:
        record_cleanup_error(receipt_path, error)
        raise


def record_cleanup_error(path, error):
    if path.exists():
        receipt = json.loads(path.read_text(encoding="utf-8"))
        if not receipt.get("cleaned"):
            receipt["cleanup_error"] = str(error)
            write_receipt(path, receipt)


def run(args):
    project = args.project.resolve(strict=True)
    main = Path(worktrees(project)[0]["worktree"]).resolve()
    if main != project or git(project, "rev-parse", "--is-bare-repository") != "false":
        raise IntegrationError("project must be the main Git worktree root")
    state = Path(git(project, "rev-parse", "--path-format=absolute", "--git-common-dir")) / "slate-integrations"
    if args.action == "pending":
        receipts = (json.loads(path.read_text(encoding="utf-8")) for path in sorted(state.glob("*.json")))
        return [receipt for receipt in receipts if not receipt.get("cleaned")]
    local_action = args.action.startswith("cleanup-")
    if not all((args.card_id, args.key, args.branch, args.card_json)) or (not local_action and not args.approval):
        raise IntegrationError("card ID, key, branch, approval and card JSON are required")
    if not re.fullmatch(r"[A-Z][A-Z0-9]+-[1-9][0-9]*", args.key):
        raise IntegrationError("expected a Slate card key")
    if args.card_id <= 0 or (args.approval is not None and args.approval <= 0):
        raise IntegrationError("card and approval IDs must be positive")
    if not re.fullmatch(rf"slate/{args.key.lower()}-[a-z0-9][a-z0-9-]*", args.branch):
        raise IntegrationError("expected a card-specific Slate branch")
    if local_action and not state.is_dir():
        return {"status": "unknown"}
    state.mkdir(exist_ok=True)
    task_path = project.with_name(f"{project.name}-{args.key.lower()}")
    branch_ref = f"refs/heads/{args.branch}"
    receipt_path = state / f"{args.card_id}-{args.approval}.json"
    with repository_lock(state / "lock"):
        if local_action:
            if task_path.is_symlink():
                return {"status": "blocked", "reason": "expected task worktree may not be a symlink"}
            return cleanup_local(args, project, state, task_path, branch_ref)
        if args.action == "cleanup":
            try:
                if task_path.is_symlink():
                    raise IntegrationError("expected task worktree may not be a symlink")
                return cleanup(args, project, receipt_path, task_path, branch_ref)
            except (IntegrationError, OSError) as error:
                record_cleanup_error(receipt_path, error)
                raise
        if task_path.is_symlink():
            raise IntegrationError("expected task worktree may not be a symlink")
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
    apply_parser = actions.add_parser(
        "apply", description="Supports six-character approval ranges and legacy full-hash approvals.",
    )
    apply_parser.add_argument("--base", required=True)
    apply_parser.add_argument("--reviewed-head", required=True)
    actions.add_parser("cleanup")
    actions.add_parser("cleanup-status")
    actions.add_parser("cleanup-preview")
    actions.add_parser("cleanup-retry")
    actions.add_parser("cleanup-archive").add_argument("--preview-token", required=True)
    actions.add_parser("pending")
    try:
        print(json.dumps(run(parser.parse_args())))
    except (IntegrationError, OSError, ValueError) as error:
        print(f"Slate integration: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
