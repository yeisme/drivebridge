import { useState } from 'react';
import { AppSheet } from '../../components/AppSheet';
import { Button } from '../../components/ui/button';
import type { components } from '../../generated/personal-workspace';

type Invocation = components['schemas']['InvocationPageEnvelope']['data']['items'][number];

export function AgentInvocationDetail({ invocation, observedAt, resultLabel }: {
  invocation: Invocation; observedAt: string; resultLabel: string;
}) {
  const [open, setOpen] = useState(false);
  return <AppSheet title="调用记录详情" open={open} onOpenChange={setOpen}
    trigger={<Button variant="outline" className="min-h-11" aria-label={`查看调用详情 ${invocation.invocation_id}`}>查看详情</Button>}>
    <div className="min-w-0 space-y-5 text-sm">
      <section aria-label="原记录身份" className="space-y-2">
        <h3 className="font-semibold">原记录身份</h3>
        <p>调用观察记录，不是可执行任务。服务端未提供原任务 kind，不将 action 或协议猜作 Operation / Plan 类型。</p>
        <dl className="grid gap-2 break-all">
          <div><dt>原调用 ID</dt><dd>{invocation.invocation_id}</dd></div>
          <div><dt>调用方 Agent（actor）</dt><dd>{invocation.agent_id}</dd></div>
          <div><dt>原请求 ID</dt><dd>{invocation.request_id}</dd></div>
          <div><dt>协议</dt><dd>{invocation.protocol}</dd></div>
          <div><dt>动作（action）</dt><dd>{invocation.action}</dd></div>
        </dl>
      </section>
      <section aria-label="授权事实" className="space-y-2">
        <h3 className="font-semibold">授权事实</h3>
        <dl className="break-all"><dt>调用时关联的原授权 ID</dt><dd>{invocation.grant_id}</dd></dl>
        <p>这是记录关联的授权，不代表当前授权仍有效；本记录不提供当前权限、期限或连接健康状态。单次失败不表示授权已撤销。</p>
      </section>
      <section aria-label="观察事实" className="space-y-2">
        <h3 className="font-semibold">观察事实</h3>
        <p>best_effort：非全量观察，不是完整审计。</p>
        <dl className="grid gap-2 break-all">
          <div><dt>开始时间</dt><dd>{invocation.started_at}</dd></div>
          <div><dt>观察返回时间</dt><dd>{invocation.finished_at || '未观察到返回时间（不代表仍在运行）'}</dd></div>
          <div><dt>最近列表回读时间</dt><dd>{observedAt}</dd></div>
        </dl>
      </section>
      <section aria-label="结果事实" className="space-y-2">
        <h3 className="font-semibold">结果事实</h3>
        <p>{resultLabel}（{invocation.result}）</p>
        {invocation.result === 'unknown' && <p>业务效果尚未独立证明；请在原业务入口回查，不要重新执行调用。</p>}
        <p>传输返回不等于业务完成，本详情不会执行、重试调用或创建任务。</p>
        {invocation.error_code && <dl className="break-all"><dt>安全错误码</dt><dd>{invocation.error_code}</dd></dl>}
      </section>
      <section aria-label="已证明的业务关联" className="space-y-2">
        <h3 className="font-semibold">已证明的业务关联</h3>
        {/* The current owner projection deliberately emits no business refs. */}
        <p>不可用：服务端未提供经证明的原 operation 引用，不从请求 ID、动作或时间推断业务操作。</p>
        <p>不可用：服务端未提供经证明的资源引用，不猜测文件或版本。</p>
      </section>
    </div>
  </AppSheet>;
}
