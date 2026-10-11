import type { components } from '../../generated/personal-workspace';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { SelectAdapter } from '../../components/SelectAdapter';
export type Rule = components['schemas']['CollectionRule'];
export type Filter = components['schemas']['CollectionRuleFilter'];
export type Sort = components['schemas']['CollectionRuleSort'];
export type RuleDraft = { field: Filter['field']; op: Filter['op']; value: string };
export const fields: Record<Filter['field'], string> = { path_prefix: '路径', mime_type: 'MIME 类型', caller: '调用来源', task_ref: '任务标识', 'labels.manual': '人工标签', 'labels.rule': '规则标签', 'labels.suggested': '建议标签', updated_at: '更新时间', size_bytes: '大小（字节）', trashed: '未回收文件' };
const operators: Record<Filter['op'], string> = { prefix: '目录前缀', contains: '包含', eq: '等于', ne: '不等于', gt: '大于', gte: '大于等于', lt: '小于', lte: '小于等于' };
export function allowedOps(field: Filter['field']): Filter['op'][] {
  if (field === 'path_prefix') return ['contains', 'prefix'];
  if (field.startsWith('labels.')) return ['eq', 'ne', 'contains'];
  if (field === 'size_bytes' || field === 'updated_at') return ['gt', 'gte', 'lt', 'lte'];
  return ['eq'];
}
export function compileRule(space: string, rows: RuleDraft[], sort: Sort): Rule {
  if (!space || rows.length > 64 || !['updated_at', 'name', 'path_prefix', 'size_bytes'].includes(sort.field) || !['asc', 'desc'].includes(sort.direction)) throw new Error('invalid_rule');
  const filters: Filter[] = rows.map(row => {
    if (!Object.hasOwn(fields, row.field) || !allowedOps(row.field).includes(row.op) || new TextEncoder().encode(row.value).length > 4096 || row.value.includes('\0') || new TextDecoder().decode(new TextEncoder().encode(row.value)) !== row.value) throw new Error('invalid_rule');
    let value: string | number | false = row.value;
    if (row.field === 'size_bytes') {
      if (!/^(0|[1-9]\d*)$/.test(row.value) || !Number.isSafeInteger(Number(row.value))) throw new Error('invalid_size');
      value = Number(row.value);
    } else if (row.field === 'updated_at') {
      if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(row.value) || !Number.isFinite(Date.parse(row.value))) throw new Error('invalid_time');
    } else if (row.field === 'trashed') value = false;
    else if ((row.field === 'path_prefix' || row.field.startsWith('labels.')) && !row.value) throw new Error('empty_value');
    // Field and operator pairing was validated above; the owner rechecks it.
    return { field: row.field, op: row.op, value } as Filter;
  });
  const rule: Rule = { mode: 'exact', space_ids: [space], filters, sort: [{ ...sort }] };
  if (new TextEncoder().encode(JSON.stringify(rule)).length > 64 * 1024) throw new Error('rule_budget');
  return rule;
}
export function RuleFields({ rows, sort, change, changeSort }: { rows: RuleDraft[]; sort: Sort; change: (rows: RuleDraft[]) => void; changeSort: (sort: Sort) => void }) {
  const patch = (index: number, value: Partial<RuleDraft>) => change(rows.map((row, i) => i === index ? { ...row, ...value } : row));
  return <div className="space-y-3">
    <p>所有条件同时满足。无条件时匹配当前空间全部可见的未回收文件；规则成员随文件变化，不是固定快照。不读取正文，不执行脚本。</p>
    {rows.map((row, index) => <fieldset key={index} className="space-y-3 rounded-lg border border-border p-3"><legend>规则条件 {index + 1}</legend>
      <label className="grid gap-2">字段<SelectAdapter aria-label={`条件 ${index + 1} 字段`} className="min-h-11" value={row.field} onChange={event => { const field = event.target.value as Filter['field']; patch(index, { field, op: allowedOps(field)[0], value: '' }); }}>{Object.entries(fields).map(([field, label]) => <option key={field} value={field}>{label}</option>)}</SelectAdapter></label>
      <label className="grid gap-2">操作<SelectAdapter aria-label={`条件 ${index + 1} 操作`} className="min-h-11" value={row.op} onChange={event => patch(index, { op: event.target.value as Filter['op'] })}>{allowedOps(row.field).map(op => <option key={op} value={op}>{operators[op]}</option>)}</SelectAdapter></label>
      {row.field === 'trashed' ? <p>固定 false：仅未回收文件。</p> : <label className="grid gap-2">值<Input aria-label={`条件 ${index + 1} 值`} className="min-h-11" maxLength={4096} value={row.value} onChange={event => patch(index, { value: event.target.value })}/></label>}
      {row.field === 'updated_at' && <p className="text-xs">RFC3339，含时区，例如 2026-01-01T00:00:00Z。</p>}
      {row.field === 'path_prefix' && row.op === 'prefix' && <p className="text-xs">匹配此目录下的路径，不包含目录同名文件；% 和 _ 按字面处理。</p>}
      {row.field.startsWith('labels.') && <p className="text-xs">标签为完整标签匹配；“包含”表示具有此标签，不是标签子串。</p>}
      <Button type="button" variant="outline" className="min-h-11" onClick={() => change(rows.filter((_, i) => i !== index))}>移除条件 {index + 1}</Button>
    </fieldset>)}
    <Button type="button" variant="outline" className="min-h-11" disabled={rows.length >= 64} onClick={() => change([...rows, { field: 'path_prefix', op: 'contains', value: '' }])}>添加规则条件</Button>
    <label className="grid gap-2">排序字段<SelectAdapter aria-label="规则排序字段" className="min-h-11" value={sort.field} onChange={event => changeSort({ ...sort, field: event.target.value as Sort['field'] })}><option value="updated_at">更新时间</option><option value="name">路径名称</option><option value="path_prefix">完整路径</option><option value="size_bytes">大小（字节）</option></SelectAdapter></label>
    <label className="grid gap-2">排序方向<SelectAdapter aria-label="规则排序方向" className="min-h-11" value={sort.direction} onChange={event => changeSort({ ...sort, direction: event.target.value as Sort['direction'] })}><option value="desc">降序</option><option value="asc">升序</option></SelectAdapter></label>
    <p className="text-xs text-muted-foreground">最多 64 条条件、1 个排序，同值按文件标识排序；日期与类型由服务端再次验证。</p>
  </div>;
}
