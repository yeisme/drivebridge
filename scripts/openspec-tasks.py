#!/usr/bin/env python3
"""Maintain CLI-authored task state inside an existing OpenSpec change."""
import argparse
import pathlib
import re


def new_tasks(parser, tasks, seen):
    lines = []
    for task in tasks:
        task_id, separator, text = task.partition(":")
        if not separator or not re.fullmatch(r"[a-zA-Z0-9_.-]+", task_id) or task_id in seen or "\n" in text or "\r" in text:
            parser.error("tasks require unique ID:text values")
        seen.add(task_id)
        lines.append(f"- [ ] {task_id} {text}")
    return lines


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=("init", "set", "amend", "append"))
    parser.add_argument("--change", required=True)
    parser.add_argument("--task", action="append", default=[])
    parser.add_argument("--id")
    parser.add_argument("--text", help="replacement task prose for amend; state and evidence are preserved")
    parser.add_argument("--done", action="store_true")
    parser.add_argument("--evidence", default="")
    parser.add_argument("--clear-notes", action="store_true", help="remove indented notes below the selected task")
    args = parser.parse_args()
    root = pathlib.Path(args.change)
    legacy_change = (root / "proposal.md").is_file() and (root / "specs").is_dir() and (root / "tasks.md").is_file()
    if not (root / ".openspec.yaml").is_file() and not (args.action in ("set", "amend", "append") and legacy_change):
        parser.error("create the change with openspec new change first")
    target = root / "tasks.md"
    if target.is_symlink():
        parser.error("task file must not be a symlink")
    if args.action in ("amend", "append") and (args.done or args.evidence or args.clear_notes):
        parser.error("amend and append cannot change task state, evidence, or notes")
    if args.action == "init":
        if target.exists():
            parser.error("task file already exists")
        lines = ["# 实施与验收任务", "", "由 scripts/openspec-tasks.py 维护状态。", ""]
        lines.extend(new_tasks(parser, args.task, set()))
        target.write_text("\n".join(lines) + "\n")
    elif args.action in ("amend", "append"):
        if not target.is_file():
            parser.error("task file does not exist")
        # Preserve original line endings and all prose outside the selected task.
        original = target.read_bytes().decode("utf-8")
        matches = list(re.finditer(r"^- \[[ x]\] ([a-zA-Z0-9_.-]+) ([^\r\n]*)", original, re.MULTILINE))
        ids = [match.group(1) for match in matches]
        if len(ids) != len(set(ids)):
            parser.error("existing task IDs must be unique")
        if args.action == "append":
            if not args.task or args.id or args.text is not None:
                parser.error("append requires --task ID:text")
            additions = new_tasks(parser, args.task, set(ids))
            ending = "\r\n" if "\r\n" in original else "\n"
            updated = original + ("" if original.endswith("\n") else ending) + ending.join(additions) + ending
        else:
            if not args.id or not args.text or not args.text.strip() or any(c in args.text for c in ("\r", "\n")) or args.task:
                parser.error("amend requires --id and nonempty single-line --text")
            if " | evidence:" in args.text or "<!--" in args.text:
                parser.error("amend text must not contain evidence or historical comments")
            selected = [match for match in matches if match.group(1) == args.id]
            if not selected:
                parser.error("task ID was not found")
            match = selected[0]
            body = match.group(2)
            boundaries = [body.find(marker) for marker in (" | evidence:", "<!--") if marker in body]
            tail = ""
            if boundaries:
                boundary = min(boundaries)
                while boundary > 0 and body[boundary - 1] in " \t":
                    boundary -= 1
                tail = body[boundary:]
            updated = original[:match.start(2)] + args.text.rstrip() + tail + original[match.end(2):]
        target.write_bytes(updated.encode("utf-8"))
    else:
        if not args.id or "\n" in args.evidence:
            parser.error("a task ID and single-line evidence are required")
        lines = target.read_text().splitlines()
        pattern = re.compile(r"^- \[[ x]\] " + re.escape(args.id) + r" ")
        for index, line in enumerate(lines):
            if pattern.match(line):
                text = line.split(" | evidence:", 1)[0]
                text = re.sub(r"^- \[[ x]\]", "- [x]" if args.done else "- [ ]", text)
                if args.evidence:
                    text += " | evidence: " + args.evidence
                lines[index] = text
                if args.clear_notes:
                    cursor = index + 1
                    while cursor < len(lines) and (not lines[cursor].strip() or lines[cursor].startswith("  ")):
                        cursor += 1
                    del lines[index + 1:cursor]
                break
        else:
            parser.error("task ID was not found")
        target.write_text("\n".join(lines) + "\n")
    print("task_state=updated")


if __name__ == "__main__":
    main()
