#!/usr/bin/env python3
"""Generate a read-only-source implementation inventory, never an acceptance claim."""
import argparse
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPORT = Path('docs/delivery/implementation-inventory.md')

# Approved product surfaces; routes/components are observed separately below.
SURFACES = (
    ('工作台', 'home', 'web/src/info-workspace/InfoWorkspace.tsx', 'F21.1/B03.1'),
    ('网盘', 'files', 'web/src/features/files/FilesPage.tsx', 'F04.1/F04.2'),
    ('上传与恢复', 'flows', 'web/src/consumer-flows/ConsumerFlows.tsx', 'F05.1/F05.2'),
    ('知识库', 'knowledge', 'web/src/features/notebook/KnowledgePage.tsx', 'F07.1/F25.2'),
    ('文档编辑', 'knowledge', 'web/src/features/notebook/MarkdownFileDocument.tsx', 'F07.2/F26.1/B17.2'),
    ('资源库', 'resources', 'web/src/features/resources/ResourcesPage.tsx', 'F08/B10'),
    ('集合', 'collections', 'web/src/features/collections/CollectionsPage.tsx', 'F09.1/B09.1'),
    ('标签与属性', 'properties', 'web/src/features/properties/PropertiesPage.tsx', 'F09.1/B09.2'),
    ('搜索', 'search', 'web/src/consumer-flows/FlowPage.tsx', 'F09.2'),
    ('Agent', 'agents', 'web/src/features/agents/LegacyAgentPage.tsx', 'F10/F20/B04/B15'),
    ('任务', 'tasks', 'web/src/features/tasks/LegacyTransfersPage.tsx', 'F11/B03.2'),
    ('收件', 'inbox', 'web/src/features/tasks/LegacyInboxPage.tsx', 'F12/B07/B08'),
    ('存储', 'storage', 'web/src/features/storage/ConnectionsPage.tsx', 'F13'),
    ('设备与离线', 'devices', 'web/src/features/devices/DevicesPage.tsx', 'F15.1'),
    ('自动化', 'automation', 'web/src/features/automation/AutomationPage.tsx', 'F15.2'),
    ('回收站', 'trash', 'web/src/features/trash/TrashPage.tsx', 'F21.2'),
    ('能力说明', 'capabilities', 'web/src/features/capabilities/CapabilitiesPage.tsx', 'F21.2/B11'),
    ('设置', 'settings', 'web/src/PersonalSettings.tsx', 'F21.2'),
    ('分享', 'shares', 'web/src/Sharing.tsx', 'F14'),
    ('团队', 'teams', 'web/src/Teams.tsx', 'F16.1'),
    ('治理', 'governance', 'web/src/Governance.tsx', 'F16.1'),
    ('访问凭据', 'members', 'web/src/KeyDirectory.tsx', 'F16.1'),
)
CHANGES = (
    'drivebridge-personal-workspace-rebuild-v1',
    'drivebridge-personal-workspace-contracts-v1',
    'drivebridge-desktop-knowledge-reader-v1',
)
ROUTE = re.compile(r'mux\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) ([^"\s]+)"')
TEST = re.compile(r'^func (Test\w+)\(t \*testing\.T\)', re.MULTILINE)


def text(root, path):
    return (root / path).read_text(encoding='utf-8')


def task_counts(root, change):
    lines = re.findall(r'^- \[([ x])\] (\S+)', text(root, f'openspec/changes/{change}/tasks.md'), re.MULTILINE)
    return sum(state == 'x' for state, _ in lines), len(lines)


def literal_routes(root):
    routes = {}
    for path in sorted((root / 'server/internal/service').glob('*.go')):
        if path.name.endswith('_test.go'):
            continue
        for method, route in ROUTE.findall(path.read_text(encoding='utf-8')):
            routes[(method, route)] = path.relative_to(root).as_posix()
    return routes


def test_names(root):
    tests = []
    for directory in ('server/internal/service', 'server/sdk'):
        for path in sorted((root / directory).glob('*_test.go')):
            for name in TEST.findall(path.read_text(encoding='utf-8')):
                if name.startswith(('TestWorkspace', 'TestDocument')):
                    tests.append((path.relative_to(root).as_posix(), name))
    return tests


def evidence_facts(root, run_id):
    if not re.fullmatch(r'\d{8}T\d{6}Z-[A-Za-z0-9-]+', run_id):
        raise ValueError('Invalid evidence run id')
    directory = root / 'temp/integration-test-runs' / run_id
    missing = [name for name in ('summary.json', 'command.txt', 'stdout.log', 'stderr.log', 'env.json', 'artifacts') if not (directory / name).exists()]
    if missing:
        raise ValueError('Incomplete evidence bundle')
    summary = json.loads((directory / 'summary.json').read_text(encoding='utf-8'))
    if summary.get('project') != 'cli/drivebridge' or summary.get('run_id') != run_id:
        raise ValueError('Evidence owner or identity mismatch')
    # Do not copy arbitrary commands/log text or env values into tracked docs.
    if summary.get('status') != 'passed' or summary.get('exit_code') != 0:
        raise ValueError('Evidence is not a passed check')
    stdout = (directory / 'stdout.log').read_text(encoding='utf-8')
    passed = re.findall(r'^--- PASS: (Test(?:Workspace|Document)\w+)', stdout, re.MULTILINE)
    return sorted(set(passed))


def render(root, run_id=None):
    source = text(root, 'web/src/WorkspaceApp.tsx')
    nav_block = source.split('const navigation:', 1)[1].split('function App()', 1)[0]
    pages = set(re.findall(r'\["([a-z]+)",', nav_block))
    # These three canonical aliases are explicitly implemented in workspace-route.ts.
    aliases = {'home': 'overview', 'tasks': 'transfers', 'storage': 'connections'}
    output = [
        '# DriveBridge 实现盘点（脚本生成）', '',
        '由 `python3 scripts/implementation_inventory.py --write` 生成；不可手改状态。', '',
        '## 判读规则', '',
        '- 这是静态源码和已有验收任务的盘点，不是运行时 capability 或发布承诺。',
        '- “已发现”只证明指定代码/字面路由存在；不证明可达、授权正确、视觉一致或完整闭环。',
        '- 专属页面缺失不代表旧界面没有同类操作；必须迁移时保留旧功能。',
        '- 任务计数不是完成百分比；细分任务通过不自动关闭父任务。',
        '- Electron/Pinax/三平台须独立真实验收，不能由普通文件或 fixture 通过推导。', '',
        '## 页面 → 正式路由 → 当前组件 → 验收责任', '',
        '| 页面/操作组 | 当前入口 | 指定组件 | 验收任务 |',
        '| --- | --- | --- | --- |',
    ]
    for name, route, component, tasks in SURFACES:
        entry = f'`#/{route}`' if aliases.get(route, route) in pages else '尚未发现专属导航入口'
        status = '已发现' if (root / component).is_file() else '尚未发现'
        output.append(f'| {name} | {entry} | {status}：`{component}` | {tasks} |')
    output += ['', '## 新工作区合同 → 字面 HTTP 注册 → Web 消费线索', '',
               '以下扫描只覆盖现有 Go Route 字面声明和 `mux.HandleFunc`；动态注册不在此静态扫描范围。', '',
               '| 方法/路径 | 服务注册文件 | 非生成 Web 路径字面量 |', '| --- | --- | --- |']
    registered = literal_routes(root)
    web_sources = [(path.relative_to(root).as_posix(), path.read_text(encoding='utf-8'))
                   for path in sorted((root / 'web/src').rglob('*'))
                   if path.suffix in ('.ts', '.tsx') and '/generated/' not in path.as_posix()
                   and '.test.' not in path.name]
    contracts = re.findall(r'Method: "([a-z]+)", Path: "([^"]+)"', text(root, 'server/internal/contract/routes.go'))
    for method, route in sorted(contracts):
        server = registered.get((method.upper(), route))
        # Prefix matches are navigation clues, not proof of method or semantic parity.
        prefix = route.split('{', 1)[0].removeprefix('/api/v1')
        consumers = [path for path, body in web_sources if prefix in body]
        output.append(f'| `{method.upper()} {route}` | {"`"+server+"`" if server else "未在字面注册中发现"} | {", ".join("`"+p+"`" for p in consumers) or "未发现（不含生成类型；动态构造需人工复核）"} |')
    output += ['', '## 既有任务状态（按 owner CLI 维护）', '', '| change | 已勾选/总数 |', '| --- | --- |']
    for change in CHANGES:
        done, total = task_counts(root, change)
        output.append(f'| `{change}` | {done}/{total} |')
    output += ['', f'Electron `desktop/`：{"目录存在，尚需逐项验收" if (root / "desktop").is_dir() else "目录尚不存在"}。', '',
               '## 定向测试发现与证据', '']
    names = test_names(root)
    output.append(f'静态发现 {len(names)} 个 `TestWorkspace*` / `TestDocument*` 函数；存在测试不表示测试通过。')
    if run_id:
        passed = evidence_facts(root, run_id)
        output += [f'证据：`temp/integration-test-runs/{run_id}/`；六件套存在，summary passed/exit=0。',
                   f'从 verbose 日志发现 {len(passed)} 个不同顶层测试通过；零命中不能当业务验收。']
    else:
        output.append('本报告未附本次运行证据；历史记录以任务与 owner evidence 为准。')
    output += ['', '| 测试文件 | 已声明测试 |', '| --- | --- |']
    for path, name in names:
        output.append(f'| `{path}` | `{name}` |')
    output += ['', '## 下一执行顺序', '',
               '1. 核对上表新后端已有实现及未消费端点；按原任务补缺失测试，不直接重写。',
               '2. D01：扩展同一合同生成器的 desktop profile、固定IPC与来源/阅读引用；writer另立消费任务。',
               '3. D03：证明全入口受管保护与旧writer降级拒绝；无法证明覆盖就拒绝绑定。',
               '4. D02/D04：本机门面、监督进程、共享阅读状态；D05/D06/D07：同一Web与Electron。',
               '5. 真实fixture纵切通过后，按文件、Agent/收件、知识资源、管理页四组完成全站，再做视觉和三平台门。', '']
    return '\n'.join(output)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group()
    group.add_argument('--write', action='store_true')
    group.add_argument('--check', action='store_true')
    parser.add_argument('--evidence-run')
    args = parser.parse_args()
    try:
        report = render(ROOT, args.evidence_run)
    except (ValueError, OSError, KeyError, IndexError) as error:
        parser.exit(1, f'Inventory input validation failed: {type(error).__name__}\n')
    target = ROOT / REPORT
    if args.check:
        if not target.is_file() or target.read_text(encoding='utf-8') != report:
            parser.exit(1, 'Implementation inventory drifted; regenerate with the same evidence run.\n')
        print('Implementation inventory matches source discovery.')
    elif args.write:
        if target.is_symlink():
            parser.exit(1, 'Refusing a linked report target.\n')
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(report, encoding='utf-8')
        print(f'Generated {REPORT.as_posix()}')
    else:
        print(report, end='')


if __name__ == '__main__':
    main()
