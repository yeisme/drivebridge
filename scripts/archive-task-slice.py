#!/usr/bin/env python3
"""Extract explicitly selected completed tasks into a CLI-created archive slice."""
import argparse
import pathlib
import re


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', required=True, type=pathlib.Path)
    parser.add_argument('--target', required=True, type=pathlib.Path)
    parser.add_argument('--archive-name', required=True)
    parser.add_argument('--id', action='append', required=True)
    args = parser.parse_args()
    source, target = args.source.resolve(), args.target.resolve()
    if source.parent != target.parent or not (target / '.openspec.yaml').is_file():
        parser.error('Target must be a CLI-created sibling change.')
    if pathlib.Path(args.archive_name).name != args.archive_name:
        parser.error('Archive name must be a basename.')
    tasks = source / 'tasks.md'
    if tasks.is_symlink() or (target / 'tasks.md').exists():
        parser.error('Refusing symlink or existing target tasks.')
    original = tasks.read_text()
    lines = original.splitlines(keepends=True)
    selected = {}
    for index, line in enumerate(lines):
        match = re.match(r'^- \[([ xX])\] (\S+) ', line)
        if match and match[2] in args.id:
            if match[1].lower() != 'x' or match[2] in selected or ' | evidence:' not in line:
                parser.error('Selected tasks must be unique, completed and evidenced.')
            if index + 1 < len(lines) and lines[index + 1].startswith(('  ', '\t')):
                parser.error('Indented task notes require explicit handling.')
            selected[match[2]] = (index, line)
    if set(selected) != set(args.id) or len(args.id) != len(set(args.id)):
        parser.error('Every selected ID must exist exactly once.')
    for task_id, (index, _) in selected.items():
        lines[index] = f'- 已归档 {task_id}：见 [已验收切片](../archive/{args.archive_name}/tasks.md)；原编号、依赖与完整证据保存在归档中。\n'
    snapshot = target / 'source-tasks-before-split.md'
    snapshot.write_text(original)
    (target / 'tasks.md').write_text('# 已验收切片任务\n\n由 scripts/archive-task-slice.py 从原任务表提取；原编号及证据保持不变。I109首次完成被否决，最终以C110–V114及独立审计为准。\n\n' + ''.join(line for _, line in selected.values()))
    if tasks.read_text() != original:
        raise RuntimeError('Source changed during extraction; source was not modified.')
    tasks.write_text(''.join(lines))
    print(f'Extracted {len(selected)} completed tasks; untouched tasks and pre-split snapshot retained.')


if __name__ == '__main__':
    main()
