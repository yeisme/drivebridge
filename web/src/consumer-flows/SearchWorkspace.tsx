import { useEffect, useState } from 'react';
import type { Space } from '../api';
import { Button } from '../components/ui/button';
import { SearchCenter } from './SearchCenter';
import { probeFlowCapability, type FlowCapability } from './client';
import './consumer-flows.css';

// Canonical page and command dialog share the same search-only boundary.
// Metadata is independently authorized even when content discovery is absent.
export function SearchWorkspace({ spaces }: { spaces: Space[] }) {
  const [capability, setCapability] = useState<FlowCapability>('checking');
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setCapability('checking'); setFailed(false);
    probeFlowCapability('content_query', controller.signal).then(value => {
      if (!controller.signal.aborted) setCapability(value);
    }).catch(() => { if (!controller.signal.aborted) setFailed(true); });
    return () => controller.abort();
  }, [attempt]);
  return <>
    {failed && <div role="alert" className="space-y-2 text-sm"><p>无法读取内容发现能力，请重试；元数据查询仍会独立鉴权。</p><Button type="button" variant="outline" className="max-md:min-h-11" onClick={() => setAttempt(value => value + 1)}>重试</Button></div>}
    <SearchCenter spaces={spaces} capability={capability}/>
  </>;
}
