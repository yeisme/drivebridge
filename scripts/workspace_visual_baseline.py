#!/usr/bin/env python3
"""Generate a source-protection manifest and a scoped visual comparison report."""
import argparse
import hashlib
import html
import json
import os
import re
import shlex
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SURFACES = (
    ('home', 'overview', '首页', 'WorkspaceHome + InfoWorkspace 同屏叠加；改为个人首页主路径，旧观察视图保留独立兼容入口。'),
    ('files', 'files', '文件', '默认仍先选网盘；复用真实 DriveHome/Finder，统一目录、工具条、列表、详情和上传恢复，不导入原型样本。'),
    ('knowledge', 'knowledge', '知识与笔记', '复用正式 NotebookWorkspace 与普通文件保存；对齐布局、标签页和阅读，Pinax 真实连接另验。'),
    ('resources', 'resources', '资源库', '保留已有描述登记/固定版本/检查能力，收敛大卡片和重复说明，资源保存不等于安装执行。'),
    ('collections', 'collections', '集合', '保留手动/规则集合、成员与快照，改紧凑索引；移除成员不删除原文件。'),
    ('properties', 'properties', '标签属性', '保留已有字典、提案与冲突保护；统一表单和列表，不能直接批写 YAML。'),
    ('search', 'search', '搜索', '保留全页深链与命令搜索，补齐稳定成功/空/覆盖不足/错误对照，不以查询中当完成。'),
    ('agents', 'agents', 'Agent', '新旧授权同屏且存在两个创建入口；新版为默认，旧协议管理显式切换，保留独立权限语义。'),
    ('tasks', 'transfers', '任务', '投影与旧传输列表同屏；统一任务入口，旧上传恢复操作保留可见入口，不能因外观隐藏待恢复记录。'),
    ('inbox', 'inbox', '收件', 'submission 与旧任务收件重复空态；按来源明确切换，验收、归档、传输仍为不同事实。'),
    ('storage', 'connections', '存储', '复用真实连接与能力矩阵；存储管理不得默认授予管理员权限。'),
    ('devices', 'devices', '设备离线', '路由未登记；建立明确的宿主能力入口，不伪造设备在线或浏览器后台备份。'),
    ('automation', 'automation', '备份整理', '路由未登记；复用规则/意图公开合同与现有文件流程，不新增运行引擎。'),
    ('capabilities', 'capabilities', '能力说明', '路由未登记；展示真实能力与未接入原因，页面读取不触发探测或付费。'),
    ('trash', 'trash', '回收站', '保留批量恢复、权限与读取状态，禁止永久删除；压缩冗余空态。'),
    ('settings', 'settings', '设置', '保留身份/外观/密度及受权旧管理入口，避免页面标题重复。'),
    ('shares', 'shares', '分享', '冻结导航和文件操作未观察到独立分享管理；固定引用不是分享。保留正式分享深链与固定清单，不扩大权限。'),
)


def load_observations(directory, name, key):
    try:
        value = json.loads((directory / name).read_text())[key]
        if not isinstance(value, list):
            raise ValueError('Expected an observation list.')
        return value
    except (OSError, ValueError, KeyError, TypeError) as error:
        raise ValueError('Invalid visual observation input: ' + name) from error


def protected_sources(root):
    # Hash source changes without collecting file contents, environment or credentials.
    status = subprocess.check_output(['git', 'status', '--porcelain=v1', '-z', '--untracked-files=all'], cwd=root)
    paths = subprocess.check_output(['git', 'ls-files', '--modified', '--others', '--exclude-standard', '-z'], cwd=root).decode().split('\0')
    result = []
    for relative in sorted(set(filter(None, paths))):
        target = root / relative
        if target.is_symlink():
            result.append({'path': relative, 'kind': 'symlink', 'policy': 'preserve-in-place'})
        elif target.is_file():
            result.append({'path': relative, 'sha256': hashlib.sha256(target.read_bytes()).hexdigest(), 'policy': 'preserve-in-place'})
        else:
            result.append({'path': relative, 'kind': 'missing', 'policy': 'preserve-existing-deletion'})
    return {'git_status_sha256': hashlib.sha256(status).hexdigest(), 'entries': result}


def render(reference_id, product_id, reference, product, details_id=None, details=None):
    lines = ['# 全站 Web 原型差距基线', '', '由 `scripts/workspace_visual_baseline.py` 生成；本报告是基线观察与实施决定，不是功能验收。', '',
             '整体状态：首次完成申请被独立审计否决；C110–V114的实现、稳定态补拍和逐项对照现已补充在后文，最终是否完成以独立审计为准。历史测试与旧截图不等于整体批准；旧204观察含读取中画面，不作为当前稳定态证据。', '',
             f'- 冻结原型：`temp/integration-test-runs/{reference_id}/artifacts/`（只读演示页面）。',
             f'- 本地产品：`temp/integration-test-runs/{product_id}/artifacts/`（真实 disposable fixture，未代替完整业务验证）。',
             '- 设计准入：Web/文件/授权/任务 fit；Pinax 笔记与外部生成 split-owner；复制演示状态、远端改原型与假在线 reject-now。',
             '- 原型入口及实际页面加载依赖采用双读 SHA-256 固定；只覆盖被访问页面资源，不宣称未访问路径的完整镜像。',
             '- 改动保护清单位于本次 runner 的 source-protection.json；列出的既有改动全部原路径保留，不 reset/clean，不按 Git 未跟踪状态删除迁仓源码。', '',
             '## 开工截面：页面映射与处理（历史基线，当前结果见后文）', '', '| 页面 | 原型观察 | 当时产品观察 | 决定 |', '| --- | --- | --- | --- |']
    for ref, route, name, decision in SURFACES:
        a = next((row for row in reference if row['width'] == 1440 and row['id'] == ref), None)
        b = next((row for row in product if row['width'] == 1440 and row['route'] == route), None)
        left = ' / '.join(a.get('headings', [])[-4:]) if a else '尚未单独捕获，保留补验'
        right = ' / '.join(b.get('headings', [])[:4]) if b else '尚未捕获'
        lines.append(f'| {name} | {left} | {right} | {decision} |')
    lines += ['', '## 开工时的详情与弹层检查清单（闭合结果见逐项矩阵）', '',
              '- 文件：固定版本预览、详情/版本/活动/关联、下载/分享、上传与冲突恢复。',
              '- 笔记：多标签/分屏、快速打开、命令、保存差异、冲突草稿、失权清理。',
              '- Agent：创建、一次性凭据、分组/置顶、期限/禁用/撤销/轮转、调用与原任务。',
              '- 收件/任务：固定版本、人工决定、归档目标、未知结果恢复、计划确认。',
              '- 管理：连接检查、能力说明、设备宿主限制、自动化意图、回收恢复、分享。',
              '- 原型 Agent 创建已捕获；以下详情补充按实际观察记录，其余列为后续必需截图/交互验收，不标通过。凭据显示页面禁止截图。', '',
              '## 本轮设计与兼容决定', '',
              '- 继续 Precision Pine：184/56px 侧栏、48px 顶栏、18px 页面标题、14px 正文、13px 控件、桌面32px/触控44px；复用 tokens/Base UI/Tailwind。',
              '- 页面以列表/表格/检查面板为主，不再用大容器包小卡片或重复空态堆出整屏。',
              '- 旧授权、旧收件、旧传输及总览观察改为明确兼容入口；保持挂载/恢复语义由具体实现及测试确认，不复制旧状态。',
              '- 文案短句先说明用户可做的事；授权、覆盖范围、观察时间、未知结果等必要限制仍可读，技术细节放展开说明。',
              '- 全站前端与文件/Agent两条真实本地链分别验收。设备原生、Pinax真实接入、桌面三平台、真实云和发布不在此轮。',
              '- 首页/Agent/收件先收敛重复结构，随后补三个缺失路由、其余信息页/文件阅读器，最后六视口浅深主题与核心链完整回归。', '']
    if details_id:
        lines += ['## 固定快照详情补充', '', f'证据：`temp/integration-test-runs/{details_id}/artifacts/`；浏览器只使用已冻结资源，无远端写入。', '', '| 场景 | 实际观察 | 后续验收 |', '| --- | --- | --- |']
        for row in details or []:
            observed = '观察失败，不能算通过' if row.get('error') or row.get('errors') or row.get('blocked') else '已捕获当前画面；不等于业务通过'
            if row['id'] == 'note-commands' and not row.get('dialogs'):
                observed = '快捷键后未观察到 dialog，不推断命令面板可用'
            lines.append(f"| {row['id']} | {observed} | 固定版本、焦点、实际动作与移动端独立验证 |")
        lines.append('')
    return '\n'.join(lines)


COMPARISON_SCOPE = {
    'home': ('分段摘要与旧总览分开；计数不冒执行进度', 'workspace/home、固定引用', 'workspace-home、information-layout'),
    'files': ('四视图、手机目录Sheet与固定版本大预览；保留恢复', 'files/versions、uploads、operations', 'files-layout、file-preview-layout、workspace、file-note-loop'),
    'knowledge': ('默认闭合大纲、分屏与原版本编辑；冲突保草稿', 'document-capability、document CAS/lookup', 'notebook-layout、knowledge-workspace、file-note-loop'),
    'resources': ('紧凑资源登记/列表/原快照；不安装执行', 'workspace resources、snapshot members', 'information-layout、resources、resource-snapshot'),
    'collections': ('紧凑索引、手动/规则集合、跨空间原成员和快照', 'collections、members、snapshots/lookup', 'information-layout、collection-*、snapshot-history'),
    'properties': ('字典/固定版本sidecar、原回执与建议人工采纳', 'property dictionary、file properties、label suggestions', 'information-layout、properties、file-properties-*、label-suggestions'),
    'search': ('独立搜索与命令入口，真实路径、分页、覆盖/错误分离', 'file-queries、saved conditions、search capability', 'search-*、information-layout、command-search'),
    'agents': ('目录受限创建/期限/轮转/组织、占位配置、调用与原任务', 'agent-grants、organizations、invocations、MCP/HTTP', 'agent-layout、grant-*、agent-inbox-loop、submission-isolation'),
    'tasks': ('统一原任务投影、440px可调Sheet、未知值及旧传输恢复', 'workspace/tasks、operations、upload_batch', 'task-inbox-layout、task-poll-*、batch-tasks、workspace'),
    'inbox': ('真实文件名与来源优先；原版本文本/媒体/下载、人工接受/拒绝/归档与原键恢复', 'inbox-submissions、decision、archive/lookup/resume', 'task-inbox-layout、agent-inbox-loop、submission-inbox'),
    'storage': ('紧凑连接、显式检查；历史验证不冒在线', 'connections、capabilities', 'management-layout、connections-page（连接边界替身）'),
    'devices': ('真实浏览器状态与宿主限制，不伪造设备或离线副本', 'navigator online、公开capabilities', 'workspace-baseline、workspace-management、组件失效反例'),
    'automation': ('默认关闭人工配置/有界预览/确认/原键回执，真实标签效果', 'automation management/changes/preview/execute/lookup', 'automation-management、automation-tasks、Go safety/HTTP'),
    'capabilities': ('实现/配置/权限/观察分离，检查需显式发起', 'GET capabilities、POST capabilities/check', 'workspace-baseline、workspace-management、组件反例'),
    'trash': ('列表/网格和批量恢复，原版本回读；禁止永久删除', 'trash、restore、retention', 'management-layout、trash-batch、trash-read-state'),
    'settings': ('紧凑主题/资料/账户信息，桌面大纲与手机触控', 'profile、avatar、非秘密appearance preference', 'management-layout、profile、appearance-settings'),
    'shares': ('单一空态、明确信息列、固定分享撤销与授权失效', 'private shares、revoke、fixed manifest', 'management-layout、finder、SharingPage组件反例'),
}


def render_comparison(reference_id, comparison_id, observations, evidence, supplement_id=None):
    expected = {(route, width, theme) for _, route, _, _ in SURFACES
                for width in (320, 390, 768, 1280, 1440, 1920) for theme in ('light', 'dark')}
    actual = {(row.get('route'), row.get('width'), row.get('theme')) for row in observations}
    pending = re.compile(r'(?:正在(?:读取|加载|检索)|读取[^\n。]*…|加载中|读取中)')
    invalid = any(not isinstance(row.get('overflow'), bool) or row['overflow'] or row.get('errors')
                  or not isinstance(row.get('stable'), bool) or not row['stable']
                  or not isinstance(row.get('expectedState'), str) or not row.get('expectedState')
                  or not isinstance(row.get('text'), str) or row['expectedState'] not in row['text']
                  or pending.search(row['text']) for row in observations)
    if actual != expected or len(observations) != len(expected) or invalid:
        raise ValueError('Final comparison requires 204 complete, unique, explicitly stable, error-free route observations.')
    lines = ['', '## 2026-10-08 本轮全站核对与交付范围', '',
             '以下补充不改写上方历史差距。逐页结论来自源码、截图人工复核与分层测试；截图零溢出本身不是业务验收，也不是像素相同声明。', '',
             f'- 当前全站截图：`temp/integration-test-runs/{comparison_id}/artifacts/`，17页面×6宽度×2主题＝204观察，每页明确稳定态且零横向溢出/页面异常（非整体完成批准）。',
             f'- 原型继续固定为 `{reference_id}`；比较图集见本报告生成run的 `artifacts/visual-comparison.html`（本地打开，相对链接指向冻结原图与实现）。',
             '- populated列表/详情/弹层另见下列layout测试，不能用空页面图替代；一次性凭据显示期间不截图。', '',
             '| 页面 | 本轮处理与核对结论 | 原合同依赖 | 独立验证（web/tests/*.spec.ts，除特别说明） | 桌面/手机原型与实现 |',
             '| --- | --- | --- | --- | --- |']
    for ref, route, name, _ in SURFACES:
        decision, contract, tests = COMPARISON_SCOPE[ref]
        links = []
        for width in (1440, 390):
            original_run = supplement_id if ref == 'search' else reference_id
            original = ROOT / 'temp/integration-test-runs' / (original_run or '') / 'artifacts' / f'reference-{width}-{ref}.png'
            if original.is_file():
                links.append(f'[原型{width}](../../../temp/integration-test-runs/{original_run}/artifacts/{original.name})')
            else:
                links.append(f'原型{width}：未观察到独立对应页，不宣称等价')
            for theme in ('light', 'dark'):
                links.append(f'[实现{width}/{theme}](../../../temp/integration-test-runs/{comparison_id}/artifacts/product-{theme}-{width}-{route}.png)')
        lines.append(f'| {name} | {decision} | {contract} | {tests} | {"<br>".join(links)} |')
    lines += ['', '### 详情、兼容与安全边界', '',
              '- 预览/阅读：固定版本正文与详情、信息叠层、目录/大纲、分屏、差异确认、未知原键恢复；阅读器resize/theme不重取正文。原型未观察到的笔记命令弹窗不虚构参考，使用正式组件键盘测试。',
              '- Agent/收件：新旧协议分tab；配置仅占位且零调用，真实客户端使用HTTP模板完成受限调用/提交；MCP授权另由Go真实服务测试覆盖。上传完成、调用返回、人工接收和归档保持四种事实。',
              '- 兼容：首页/旧overview、任务/旧transfers、收件legacy深链、团队/管理员/Key与本机只读view均保留；手机租户入口在工作区选项，桌面仍在顶栏。',
              '- 终验发现并修复：长任务名撑高→两行摘要加完整披露；搜索关闭延迟返焦→仅在body无焦点时补返焦；团队手机145px顶栏→48px；普通拖拽误继承版本上传→显式picker意图，原版本/续传与新文件隔离。',
              '- 测试归因：旧收件默认、旧Agent活动、68px侧栏/桌面44px假设按现行UI合同更新；分组DOM不是跨组排名，改查真实服务排序；懒加载等待Key专属控件而非旧目录同名行。测试fixture污染由独立服务重验归因，不放宽账号接管/Key权限。',
              '- scoped安全复核覆盖当前身份鉴权、限定目录、CSRF、原键/digest、CAS、事务效果/回执、迟包清除、秘密不持久化及不执行不可信内容；不是全仓渗透、依赖CVE扫描或生产安全认证。',
              '- 范围外明确保留：真实云、个人vault/Pinax受管读写、Electron三平台、移动原生/后台备份、内置模型、PostgreSQL实机、部署/发布。未运行race、未提交/推送。',
              '- 已知非阻断：构建>500kB chunk警告；部分LSP新增模块缓存诊断/静默probe未确认，完整tsc/Go编译测试另有实际证据，不称全树LSP clean。旧大型OpenSpec仍含范围外任务，不整包归档。', '',
              '### 已确认通过的证据run', '', '以下为实际子进程命令，每项有独立六件套；fixture启用环境按开发入口设置（自动化/属性/传输/团队/归档故障分别独立），个人Key与团队fixture不能混跑。含替身场景的suite不整体冒称真实外部集成。', '']
    for run_id, summary in evidence:
        command = shlex.join(summary['command']).replace('|', '\\|')
        lines.append(f'- `{run_id}`：`{command}`。')
    lines += ['', '运行入口与显式开关见 [开发入口](../../development.md)；实现规则见 [UI Spec](personal-workspace-ui-spec.md)。', '']
    return '\n'.join(lines)


# name, frozen family, frozen image template, implementation image template,
# intentional treatment, public contract, browser verification entry.
DETAILS = (
    ('工作区搜索弹层', 'supplement', 'reference-{width}-search.png', 'dialog-search-light-{width}.png', '桌面与手机分别补拍冻结搜索；正式与独立搜索页共用查询状态，默认等待输入不是加载态。', 'file-queries + search capability', 'workspace-dialog-baseline、search-owner'),
    ('文件固定版本阅读', 'supplement', 'reference-{width}-file-preview.png', 'preview-reading-light-{width}.png', '保留大阅读区；手机全屏，原型演示正文改为受权原版本，不复用样本。', 'files/media + version', 'file-preview-layout'),
    ('文件信息侧栏', 'supplement', 'reference-{width}-file-info.png', 'preview-information-light-{width}.png', '桌面360px，手机叠层；字段与操作分组，展开不卸载正文。1440信息在同页阅读截图。', 'files/stat + properties', 'file-preview-layout'),
    ('文件活动', 'supplement', 'reference-{width}-file-tab-activity.png', 'preview-versions-{width}.png', '冻结演示终态无活动；正式展示真实版本时间/来源，调用记录与原任务从独立入口查看。不编造完整文件审计时间线。', 'files/versions + workspace/tasks + invocations', 'file-preview-layout、agent-inbox-loop'),
    ('文件关联与笔记关系', 'supplement', 'reference-{width}-file-tab-links.png', 'notebook-relations-{width}.png', '普通笔记已有本页标题/块定位与受权目标解析；关系图/反链由来源公开合同负责，Pinax真实接入另验。缺合同明确未支持，不伪造零关系或推断可删除。', 'notebook source capabilities + references', 'notebook-workspace、notebook-wikilinks.test'),
    ('历史版本', 'supplement', 'reference-{width}-file-tab-history.png', 'preview-versions-{width}.png', '原型tab收敛为信息栏披露；仍可复制/下载指定版本，不覆盖head。', 'files/versions + references', 'file-preview-layout'),
    ('固定链接与引用', 'supplement', 'reference-{width}-file-links.png', 'preview-information-light-{width}.png', '定位/Agent引用不等于分享；保留明确复制固定版本动作与鉴权说明。', 'references', 'file-preview-layout'),
    ('跨空间复制', None, None, 'preview-copy-{width}.png', '保留正式受管复制合同与独立目标；原型文件操作仅作风格依据，不伪造同一弹层。', 'archive + operation', 'file-preview-layout'),
    ('创建分享', None, None, 'share-create-{width}.png', '冻结导航无独立分享页；文件“链接与引用”明确不创建分享。保留产品已有分享配置，截图不提交或显示token。', 'shares fixed manifest', 'file-preview-layout'),
    ('撤销分享确认', None, None, 'share-revoke-{width}.png', '不复制原型内存演示；明确撤销后的新请求与已下载副本区别，私有fixture实测撤销。', 'shares/revoke', 'management-layout'),
    ('笔记分屏与目录', 'reference', 'reference-{width}-knowledge.png', 'notebook-{device}.png', '默认收起大纲，把正文放首屏；手机目录Sheet、主副窗格与嵌套锚点真实可用。', 'ordinary files/content', 'notebook-workspace'),
    ('笔记快速打开', None, None, 'notebook-quick-open-{width}.png', '冻结Control+P未观察到命令dialog，不能宣称原型有；正式快速打开按当前来源列真实文件。', 'notebook identity + file query', 'notebook-workspace'),
    ('文档保存差异', 'supplement', 'reference-{width}-file-preview.png', 'note-save-diff-{width}.png', '原型本轮演示保存替换为显式before/after、CAS和人工确认；普通文件与Pinax权限不混同。', 'files/document PUT + base_version_id', 'file-note-loop'),
    ('保存结果未知恢复', None, None, 'note-save-unknown-{width}.png', '真实提交后丢响应，保留原键与内存草稿；重载只查原键，不持久化正文或重发新键。', 'files/document/lookup', 'file-note-loop'),
    ('资源登记', 'supplement', 'reference-{width}-resource-register.png', 'dialog-resource-register-light-{width}.png', '复用单一表单；先核固定目标，再登记，登记不代表安装/运行。关闭刷新后返焦且不抢新焦点。', 'resource-descriptors + lookup', 'workspace-dialog-baseline、resources'),
    ('资源固定描述', 'reference', 'reference-{width}-resources.png', 'resource-detail-{width}.png', '紧凑描述披露真实修订与目标；编码检查和领域安装明确分开。', 'resource-descriptors/{id}/checks', 'resources'),
    ('创建手动集合', 'supplement', 'reference-{width}-collection-create.png', 'dialog-collection-create-light-{width}.png', '只建私有集合；两步确认、原请求回执和关闭草稿保护替代演示增删。', 'file-collections + lookup', 'workspace-dialog-baseline、collections'),
    ('创建规则集合', 'reference', 'reference-{width}-collections.png', 'dialog-collection-rule-light-{width}.png', '规则是授权时动态计算，不冒充固定快照；与手动集合共用表单壳。', 'file-collections rules', 'workspace-dialog-baseline、collection-rule-create'),
    ('集合固定成员', 'reference', 'reference-{width}-collections.png', 'collection-fixed-member-{width}.png', '在集合详情展开观察到的原成员引用；head更新不得替换已观察版本。', 'collection members + references', 'collections'),
    ('快照确认', 'reference', 'reference-{width}-collections.png', 'snapshot-confirm-{width}.png', '预览不创建；确认按提交时可见范围捕获，丢响应查原快照，不另建副本。', 'snapshots/creations + lookup', 'collection-snapshot-create'),
    ('属性字典差异', 'reference', 'reference-{width}-properties.png', 'property-diff-{width}.png', '字典与文件值分离；稳定key/类型不直接重写，草稿、CAS和原回执保留。', 'property-definitions PUT/lookup', 'properties'),
    ('接入Agent表单', 'supplement', 'reference-{width}-agent-create.png', 'dialog-agent-create-light-{width}.png', '默认只读长期，目录/协议/动作/期限显式配置；这里只截未提交表单，不截凭据。', 'agent-grants POST/lookup', 'workspace-dialog-baseline、agent-inbox-loop'),
    ('Agent接入配置', 'reference', 'reference-{width}-agents.png', 'grant-connection-http-light-{width}.png', '新增只读占位模板；MCP控制面和HTTP数据面分开，复制不签发凭据或调用。', 'agent-grants + mcp/http contracts', 'grant-connection、agent-inbox-loop'),
    ('Agent生命周期确认', 'reference', 'reference-{width}-agents.png', 'agent-dialog-管理授权-{width}.png', '禁用可恢复、撤销不可恢复；版本与动作确认保留，原回执不冒当前状态。', 'agent-grants PATCH/revoke/lookup', 'agent-layout、grant-lifecycle'),
    ('Agent分组与置顶', 'reference', 'reference-{width}-agents.png', 'agent-dialog-分组与置顶-{width}.png', '服务端组织元数据不改变授权；失权与原键恢复保持独立。', 'agent-organizations', 'agent-layout、agent-organization-nav'),
    ('Agent凭据轮转确认', 'reference', 'reference-{width}-agents.png', 'agent-dialog-凭据管理-{width}.png', '只展示操作确认，不截凭据；轮转与延期不混同，结果未知查原代次。', 'agent-grants/rotate/lookup', 'agent-layout、grant-rotate'),
    ('真实调用观察', 'reference', 'reference-{width}-agents.png', 'agent-observations-light-{width}.png', '真实调用记录、范围和时间筛选可读；返回200不是业务效果证明。', 'workspace/invocations', 'agent-layout、agent-inbox-loop'),
    ('原操作任务详情', 'details', 'reference-detail-task-detail.png', 'tasks-detail-light-{width}.png', '440px可调Sheet/手机全宽；原任务和未知值不推演，嵌套关闭恢复焦点。', 'workspace/tasks + operations', 'task-inbox-layout、submission-isolation'),
    ('收件文本检查', 'details', 'reference-detail-inbox-review.png', 'inbox-detail-light-{width}.png', '文件名/来源/状态为主；文本转义，不执行脚本；验收与归档分开。', 'inbox-submissions + fixed content', 'task-inbox-layout、submission-inbox'),
    ('图片收件检查', 'supplement', 'reference-{width}-inbox-image.png', 'binary-preview-light-{width}.png', '真实PNG原版本复用媒体阅读器，无自动转码；完整哈希下载回读，head漂移不串内容。', 'references + files/media/content', 'submission-binary'),
    ('非文本收件列表', 'reference', 'reference-{width}-inbox.png', 'binary-inbox-light-{width}.png', '原上传operation派生文件信息，真实Agent名称与未经验证自述分离；ID默认披露。', 'SubmissionView optional display metadata', 'submission-binary、SubmissionInbox.test'),
    ('人工接收/拒绝', 'details', 'reference-detail-inbox-review.png', 'inbox-人工验收提交收件-{width}.png', '先查看原版本，再人工决定；拒绝保留文件，不复制原型退回即进回收站的演示行为。', 'inbox-submissions/decision', 'submission-inbox、submission-binary'),
    ('归档目标与恢复', 'reference', 'reference-{width}-inbox.png', 'inbox-归档已接收收件-{width}.png', '选择受权空间和目录，不隐式覆盖/删除；超时原键回查，故障继续原operation。', 'inbox-submissions/archive + lookup/resume', 'submission-inbox、submission-binary'),
    ('添加存储连接', 'supplement', 'reference-{width}-storage-add.png', 'dialog-storage-add-light-{width}.png', '原型点击只展示连接页而非同构弹层；正式保留OAuth/S3配置表单，仅验证空表单与44px，不连接真实云。', 'connections', 'workspace-dialog-baseline'),
    ('受控整理预览', 'supplement', 'reference-{width}-task-plan.png', 'automation-preview-light-{width}.png', '手机原型用任务详情图标而非桌面查看计划按钮，补拍终态；正式从备份与整理进入预览，绑定digest后确认，无后台自动执行。', 'automation-rules preview/execute/lookup', 'automation-management'),
)


def resolve_image(root, runs, template, width):
    if not template:
        return None
    name = template.format(width=width, device='desktop' if width == 1440 else 'mobile')
    for run in reversed(runs):
        candidate = Path(root) / 'temp/integration-test-runs' / run / 'artifacts' / name
        if candidate.is_file():
            return run, name
    # Desktop file information is already visible in the main reading frame.
    if template == 'preview-information-light-{width}.png' and width == 1440:
        return resolve_image(root, runs, 'preview-reading-light-{width}.png', width)
    return None


def render_details(root, reference, details, supplement, verified_runs):
    """Return Markdown + gallery; never invent an absent reference or image."""
    families = {'reference': reference, 'details': details, 'supplement': supplement}
    markdown = ['', '## 逐详情与关键弹层对照', '',
                '以下逐项关联原型依据、当前实现、处理决定、公共合同和实际证据。原型是只读演示，不是业务证明；参考缺失明确标出，不能用桌面图充当手机图。', '',
                '| 场景 | 原型依据 | 当前实现/处理决定 | 合同依赖 | 实际证据 |', '| --- | --- | --- | --- | --- |']
    gallery = []
    def md_link(image):
        run, name = image
        return f'[{name}](../../../temp/integration-test-runs/{run}/artifacts/{name})'
    def figure(image, label):
        run, name = image
        return f'<figure><figcaption>{html.escape(label)}</figcaption><img loading="lazy" src="../../{html.escape(run)}/artifacts/{html.escape(name)}"></figure>'
    for name, family, reference_template, implementation_template, decision, contract, tests in DETAILS:
        references, implementations, figures = [], [], []
        for width in (1440, 390):
            original = None
            if family and families[family] and (family != 'details' or width == 1440):
                original = resolve_image(root, [families[family]], reference_template, width)
            if original:
                references.append(md_link(original))
                figures.append(figure(original, f'Frozen basis {width}'))
            else:
                references.append(f'{width}：未观察到独立对应图；按本行决定保留/改进正式行为')
                figures.append(f'<p>{width}: no independent matching frozen view; see decision.</p>')
            implementation = resolve_image(root, verified_runs, implementation_template, width)
            if not implementation:
                raise ValueError(f'Missing implementation screenshot: {name} {width}')
            implementations.append(md_link(implementation))
            figures.append(figure(implementation, f'Implementation {width}'))
        markdown.append(f'| {name} | {"<br>".join(references)} | {decision} | `{contract}` | {tests}<br>{"<br>".join(implementations)} |')
        gallery.append(f'<section><h2>{html.escape(name)}</h2><p>{html.escape(decision)}</p><p>Contract: {html.escape(contract)} · Verification: {html.escape(tests)}</p><div>{"".join(figures)}</div></section>')
    markdown += ['', '补充说明：PDF实际像素/文字与原字节下载见 `submission-binary` 的 `binary-pdf-first-page.png`；音视频/表格继续复用现有安全阅读器，不把这些格式宣称为本轮全部实测。凭据交付、轮转结果不截图，使用字段隐藏、无持久化、实际受限调用与原请求恢复断言验证。',
                 '冻结分享核对：`链接与引用`只显示“复制定位链接/复制Agent引用/复制Agent上下文”，且明确“复制不会创建分享或扩大Agent权限”；导航与文件操作未观察到独立分享管理页。正式分享管理/创建/撤销属于既有能力保留，不拿此引用弹层冒充分享原型。', '']
    return '\n'.join(markdown), ''.join(gallery)


def validate_supplement(rows):
    if not rows or any(row.get('errors') or row.get('blocked')
                       or not isinstance(row.get('stable'), bool) or not row['stable']
                       or not isinstance(row.get('text'), str) or not row['text']
                       or re.search(r'正在(?:读取|加载|检索)|加载中|读取中', row['text']) for row in rows):
        raise ValueError('Frozen supplement must be explicitly stable without loading, page errors or uncaptured resources.')


def comparison_gallery(reference_id, comparison_id, details_id, supplement_id=None):
    sections = []
    for ref, route, name, _ in SURFACES:
        figures = []
        for width in (1440, 390):
            original = ROOT / 'temp/integration-test-runs' / reference_id / 'artifacts' / f'reference-{width}-{ref}.png'
            if original.is_file():
                figures.append(f'<figure><figcaption>Frozen reference {width}</figcaption><img loading="lazy" src="../../{reference_id}/artifacts/{original.name}"></figure>')
            elif ref == 'search' and supplement_id:
                original = ROOT / 'temp/integration-test-runs' / supplement_id / 'artifacts' / f'reference-{width}-search.png'
                if not original.is_file():
                    raise ValueError('Missing independently sized frozen search reference.')
                figures.append(f'<figure><figcaption>Frozen search dialog {width}; canonical page is an intentional extension</figcaption><img loading="lazy" src="../../{supplement_id}/artifacts/{original.name}"></figure>')
            else:
                figures.append('<p>No independent frozen screenshot; do not infer visual equivalence. 分享管理未在冻结导航与文件动作中观察到；链接与引用只提供固定引用，不创建分享。保留正式产品的独立分享管理与私有授权，不伪造原型对应页。</p>')
            for theme in ('light', 'dark'):
                figures.append(f'<figure><figcaption>Implementation {width} {theme}</figcaption><img loading="lazy" src="../../{comparison_id}/artifacts/product-{theme}-{width}-{route}.png"></figure>')
        sections.append(f'<section><h2>{html.escape(name)}</h2><div>{"".join(figures)}</div></section>')
    return '<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>DriveBridge frozen comparison</title><style>body{margin:24px;font:15px system-ui;color:#20382c;background:#f5f7f5}section>div{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}figure{margin:0;padding:12px;background:white;border:1px solid #d1ddd5}img{max-width:100%;height:auto}figcaption{margin-bottom:8px}@media(max-width:700px){section>div{grid-template-columns:1fr}}</style><h1>DriveBridge 原型 / 实现对照</h1><p>只读冻结原型；真实业务另验。当前图集先列稳定页面，随后列长列表/详情/关键弹层及有意差异。旧页面初始loading图不参与当前对照。</p>' + ''.join(sections) + '</html>'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference-run', required=True)
    parser.add_argument('--product-run', required=True)
    parser.add_argument('--write', action='store_true')
    parser.add_argument('--details-run')
    parser.add_argument('--comparison-run')
    parser.add_argument('--supplement-run')
    parser.add_argument('--verified-run', action='append', default=[])
    args = parser.parse_args()
    for value in filter(None, (args.reference_run, args.product_run, args.details_run, args.comparison_run, args.supplement_run, *args.verified_run)):
        if '/' in value or '\\' in value or value.startswith('.'):
            parser.error('Use an owner evidence run id.')
    output = os.environ.get('DRIVEBRIDGE_EVIDENCE_ARTIFACTS')
    if not output:
        parser.error('Run through scripts/check.py.')
    reference = load_observations(ROOT / 'temp/integration-test-runs' / args.reference_run / 'artifacts', 'reference-state.json', 'observations')
    product = load_observations(ROOT / 'temp/integration-test-runs' / args.product_run / 'artifacts', 'workspace-observations.json', 'observations')
    details = load_observations(ROOT / 'temp/integration-test-runs' / args.details_run / 'artifacts', 'reference-details.json', 'results') if args.details_run else None
    report = render(args.reference_run, args.product_run, reference, product, args.details_run, details)
    if args.comparison_run:
        observations = load_observations(ROOT / 'temp/integration-test-runs' / args.comparison_run / 'artifacts', 'workspace-observations.json', 'observations')
        evidence = []
        for run_id in args.verified_run:
            try:
                summary = json.loads((ROOT / 'temp/integration-test-runs' / run_id / 'summary.json').read_text())
            except (OSError, ValueError) as error:
                raise ValueError('Invalid verification receipt: ' + run_id) from error
            if summary.get('status') != 'passed' or summary.get('exit_code') != 0:
                raise ValueError('Cannot record an incomplete or failed run as verified: ' + run_id)
            evidence.append((run_id, summary))
        report += render_comparison(args.reference_run, args.comparison_run, observations, evidence, args.supplement_run)
        gallery = comparison_gallery(args.reference_run, args.comparison_run, args.details_run, args.supplement_run)
        if args.supplement_run:
            supplement = load_observations(ROOT / 'temp/integration-test-runs' / args.supplement_run / 'artifacts', 'reference-supplement.json', 'results')
            validate_supplement(supplement)
            detail_report, detail_gallery = render_details(ROOT, args.reference_run, args.details_run, args.supplement_run, args.verified_run)
            report += detail_report
            gallery = gallery.replace('</html>', detail_gallery + '</html>')
        (Path(output) / 'visual-comparison.html').write_text(gallery)
    (Path(output) / 'source-protection.json').write_text(json.dumps(protected_sources(ROOT), ensure_ascii=False, indent=2) + '\n')
    (Path(output) / 'visual-gap-baseline.md').write_text(report)
    if args.write:
        (ROOT / 'docs/web/ui/workspace-visual-gap-baseline.md').write_text(report)
    print(f'Captured protection hashes and {len(SURFACES)} surface decisions; this is not acceptance evidence.')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit('Baseline capture failed: ' + str(error)) from error
