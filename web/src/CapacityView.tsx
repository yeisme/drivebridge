import { Button } from "./components/ui/button";
import { AppDialog } from "./components/AppDialog";
import { SelectAdapter } from "./components/SelectAdapter";
import React, { useEffect, useRef, useState } from "react";
import { X, ArrowRight, RefreshCw } from "lucide-react";
import { api, bytes, errorText, getDataContext, type Space } from "./api";
import { capacityOf, percentText } from "./capacity";
import "./capacity.css";

export function CapacityBar({ space }: { space: Space }) {
  const model = capacityOf(space);
  return <div className="capacity-track" aria-label={model.percent === null ? "配额或占用未知" : `已占用 ${percentText(model.percent)}`}>
    {model.percent !== null && <><i className="capacity-used" style={{ width: `${Math.min(100, model.used! / model.quota! * 100)}%` }} /><i className="capacity-reserved" style={{ width: `${Math.min(Math.max(0, 100 - model.used! / model.quota! * 100), model.reserved! / model.quota! * 100)}%` }} /></>}
  </div>;
}

export function CapacityDonut({ space }: { space: Space }) {
  const model = capacityOf(space);
  const [active, setActive] = useState<number | null>(null);
  const parts = [
    { label: "已使用", value: model.used, tone: "used" },
    { label: "上传预留", value: model.reserved, tone: "reserved" },
    { label: "剩余可用", value: model.free, tone: "free" },
  ];
  let remaining = 100;
  let offset = 0;
  return <div className="capacity-visual">
    <div className="capacity-ring">
      <svg viewBox="0 0 240 240" role="img" aria-label={`${space.name} 空间配额；${model.percent === null ? "配额或占用未知" : "已占用 " + percentText(model.percent)}`}>
        <circle cx="120" cy="120" r="92" className="capacity-free" strokeWidth="24" fill="none" />
        {model.percent !== null && parts.map((part, i) => {
          const amount = Math.min(remaining, part.value! / model.quota! * 100);
          const start = offset; offset += amount; remaining -= amount;
          return amount > 0 && <circle key={part.tone} className={`capacity-${part.tone}`} cx="120" cy="120" r="92" fill="none" strokeWidth={active === i ? 30 : 24} pathLength="100" strokeDasharray={`${amount} ${100 - amount}`} strokeDashoffset={-start} transform="rotate(-90 120 120)" opacity={active === null || active === i ? 1 : 0.45} onMouseEnter={() => setActive(i)} onMouseLeave={() => setActive(null)} />;
        })}
      </svg>
      <div className="capacity-center"><strong>{model.percent === null ? "—" : percentText(model.percent)}</strong><span>{model.percent === null ? (model.quota === null ? "配额未提供" : "占用数据缺失") : "已占用"}</span></div>
    </div>
    <div className="capacity-legend" aria-label="空间用量明细">
      {parts.map((part, i) => <Button layout="surface" variant="ghost" type="button" key={part.tone} aria-pressed={active === i} onMouseEnter={() => setActive(i)} onMouseLeave={() => setActive(null)} onFocus={() => setActive(i)} onBlur={() => setActive(null)} onClick={() => setActive(active === i ? null : i)}><i className={`capacity-${part.tone}`} /><span>{part.label}</span><strong>{part.value === null ? "未知" : bytes(part.value)}</strong><small>{model.quota && part.value !== null ? percentText(part.value / model.quota * 100) : "—"}</small></Button>)}
    </div>
    <p className="capacity-caption">空间配额 {model.quota === null ? "未提供" : bytes(model.quota)}</p>
    {model.overflow > 0 && <p role="status" className="error">超出配额 {bytes(model.overflow)}</p>}
  </div>;
}

export function CapacityPanel({ initialSpace, onClose, onDetails, onRefresh }: {
  initialSpace?: string; onClose: () => void; onDetails: (id: string) => void; onRefresh?: (spaces: Space[]) => void;
}) {
  const [spaces, setSpaces] = useState<Space[]>([]);
  const [selected, setSelected] = useState(initialSpace || "");
  const [updated, setUpdated] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(true);
  const [revision, setRevision] = useState(0);
  const callback = useRef(onRefresh); callback.current = onRefresh;
  useEffect(() => {
    let controller: AbortController | undefined;
    const scope = getDataContext().revision;
    const load = () => {
      if (document.hidden) { controller?.abort(); return; }
      controller?.abort(); controller = new AbortController();
      const signal = controller.signal;
      setBusy(true);
      void api<Space[]>("/spaces", "GET", undefined, {}, signal).then(rows => {
        if (signal.aborted || getDataContext().revision !== scope) return;
        setSpaces(rows); setError(""); setUpdated(new Date().toLocaleTimeString("zh-CN"));
        setSelected(id => rows.some(s => s.id === id) ? id : rows[0]?.id || "");
        callback.current?.(rows);
      }).catch(e => { if (!signal.aborted && getDataContext().revision === scope) { setSpaces([]); callback.current?.([]); setUpdated(""); setError(errorText(e)); } })
        .finally(() => { if (!signal.aborted) setBusy(false); });
    };
    load(); const timer = setInterval(load, 30000);
    document.addEventListener("visibilitychange", load);
    return () => { controller?.abort(); clearInterval(timer); document.removeEventListener("visibilitychange", load); };
  }, [revision]);
  const current = spaces.find(s => s.id === selected);
  return <AppDialog title="空间用量" close={onClose}>
    {error ? <p role="alert">{error}</p> : !current ? <p role="status">{busy ? "正在读取空间用量…" : "没有可查看的空间"}</p> : <>
      <SelectAdapter aria-label="查看空间用量" value={selected} onChange={e => setSelected(e.target.value)}>{spaces.map(s => <option value={s.id} key={s.id}>{s.name}</option>)}</SelectAdapter>
      <CapacityDonut space={current}/>
    </>}
    <footer><span>{updated ? `更新于 ${updated}` : ""}</span><Button variant="ghost" size="icon" type="submit" className="icon" aria-label="刷新空间用量" disabled={busy} onClick={() => setRevision(n => n + 1)}><RefreshCw size={16}/></Button></footer>
    {current && <Button variant="outline" type="submit" className="capacity-details" onClick={() => onDetails(current.id)}>查看详细用量<ArrowRight size={16}/></Button>}
  </AppDialog>;
}
