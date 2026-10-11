import { GridTable } from "../components/DataTable";
import { Button } from "../components/ui/button";
import React, { useEffect, useRef, useState } from "react";

export function Trend({
  title,
  rows,
  fields,
  onDay,
  stacked = true,
  format = String,
}: {
  title: string;
  rows: { day: string; values: (number | null)[] }[];
  fields: string[];
  stacked?: boolean;
  format?: (n: number) => string;
  onDay: (day: string) => void;
}) {
  const [focus, setFocus] = useState("");
  const stage = useRef<HTMLDivElement>(null);
  const [measured, setMeasured] = useState(640);
  const hasData = rows.some(r => r.values.some(v => v !== null));
  useEffect(() => {
    if (!stage.current || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(([entry]) => setMeasured(Math.max(1, entry.contentRect.width)));
    observer.observe(stage.current);
    return () => observer.disconnect();
  }, [rows.length, hasData]);
  const observedMax = Math.max(
    0,
    ...rows.map((r) =>
      stacked
        ? r.values.reduce<number>((sum, v) => sum + (v || 0), 0)
        : Math.max(0, ...r.values.map((v) => v || 0)),
    ),
  );
  const max = Math.max(1, observedMax);
  const width = Math.max(1, measured - 80),
    height = 200,
    step = width / Math.max(rows.length, 1);
  const focused = rows.find((r) => r.day === focus);
  const focusIndex = rows.findIndex((r) => r.day === focus);
  return (
    <section className="insight-card">
      <h3>{title}</h3>
      <div className="chart-legend">
        {fields.map((f, i) => (
          <span key={f}>
            <i className={"swatch tone-" + i} />
            {f}
          </span>
        ))}
      </div>
      {!hasData ? (
        <p className="insight-empty">此范围未采集数据。不会用零补齐历史。</p>
      ) : (
        <>
          <div className="chart-stage" ref={stage}>
            <svg
              className="insight-chart"
              viewBox={`0 0 ${measured} ${height + 42}`}
              role="img"
              aria-label={title + "；下方提供同等数据表"}
            >
              {(observedMax === 0 ? [0] : max < 2 ? [0,1] : [0,0.5,1]).map(p => <text key={p} x="54" y={height - (height - 14) * p + 4} textAnchor="end">{format(max * p)}</text>)}
              <text x={measured - 4} y={height + 38} textAnchor="end">日期（UTC）</text>
              <g transform="translate(64,0)">
              {[0.25, 0.5, 0.75].map((p) => (
                <line
                  key={p}
                  className="chart-hairline"
                  x1="0"
                  x2={width}
                  y1={height * p}
                  y2={height * p}
                />
              ))}
              {focusIndex >= 0 && (
                <line
                  className="chart-cursor"
                  x1={focusIndex * step + step / 2}
                  x2={focusIndex * step + step / 2}
                  y1="0"
                  y2={height}
                />
              )}
              {rows.map((r, i) => {
                let offset = 0;
                return (
                  <g
                    key={r.day}
                    role="button"
                    tabIndex={0}
                    aria-label={
                      r.day +
                      " " +
                      r.values
                        .map(
                          (v, n) =>
                            fields[n] + ":" + (v === null ? "未采集" : format(v)),
                        )
                        .join("，")
                    }
                    onFocus={() => setFocus(r.day)}
                    onMouseEnter={() => setFocus(r.day)}
                    onMouseLeave={() => setFocus("")}
                    onBlur={() => setFocus("")}
                    onClick={() => onDay(r.day)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        onDay(r.day);
                      }
                    }}
                  >
                    <rect
                      x={i * step}
                      y={0}
                      width={step}
                      height={height}
                      fill="transparent"
                    />
                    {r.values.map((v, n) => {
                      const h = ((v || 0) / max) * (height - 14);
                      offset += h;
                      return v === null ? null : (
                        <rect
                          key={n}
                          x={
                            i * step +
                            2 +
                            (stacked ? 0 : (n * step) / fields.length)
                          }
                          y={height - (stacked ? offset : h)}
                          width={Math.max(
                            step / (stacked ? 1 : fields.length) - 4,
                            1,
                          )}
                          height={h}
                          rx="3"
                          ry="3"
                          className={"tone-" + n}
                        />
                      );
                    })}
                    {r.values.some(v => v !== null) && r.values.every(v => v === null || v === 0) && (
                      <line x1={i * step + 1} x2={(i + 1) * step - 1} y1={height} y2={height} className="chart-zero" />
                    )}
                    {(i === 0 || i === rows.length - 1 || i % Math.max(1, Math.ceil(rows.length / Math.max(2, Math.floor(width / 90)))) === 0) && (
                      <text
                        x={i * step + step / 2}
                        y={height + 19}
                        textAnchor={i === 0 ? "start" : i === rows.length - 1 ? "end" : "middle"}
                      >
                        {r.day.slice(5)}
                      </text>
                    )}
                  </g>
                );
              })}
              </g>
            </svg>
            {focused && (
              <div
                className="chart-float"
                role="status"
                style={{
                  left: Math.max(8, Math.min(measured - Math.min(240, measured - 16) - 8, 64 + (focusIndex + .5) * step)),
                  width: Math.min(240, measured - 16),
                }}
              >
                <strong>{focused.day}</strong>
                {focused.values.map((v, n) => (
                  <span key={fields[n]}>
                    {fields[n]} {v === null ? "未采集" : format(v)}
                  </span>
                ))}
              </div>
            )}
          </div>
          <p className="chart-readout" aria-live="polite">
            {focus
              ? rows
                  .filter((r) => r.day === focus)
                  .map(
                    (r) =>
                      r.day +
                      "：" +
                      r.values
                        .map(
                          (v, i) =>
                            fields[i] +
                            " " +
                            (v === null ? "未采集" : format(v)),
                        )
                        .join(" · "),
                  )
              : "聚焦或悬停查看数值，点击日期钻取。空白日期表示未采集或无可用记录。"}
          </p>

        </>
      )}
      {rows.length > 0 && (
          <details>
            <summary>查看数据表</summary>
            <div className="insight-table-wrap">
              <GridTable headers={[<> UTC 日期 </>,...(fields.map((f) => (<> {f} </>))),<> 查看 </>]} rows={[...(rows.map((r) => (({id:String(r.day),cells:[<> {r.day} </>,...(r.values.map((v, i) => (<> {v === null ? "未采集" : format(v)} </>))),<>
                        <Button variant="ghost" type="submit"
                          className="text-button"
                          onClick={() => onDay(r.day)}
                        >
                          明细
                        </Button>
                       </>]}))))]}/>
            </div>
          </details>
      )}
    </section>
  );
}

export function MeterBar({
  current,
  history,
  trash,
  reserved,
  quota,
  format = String,
}: {
  current: number;
  history: number | null;
  trash: number;
  reserved: number;
  quota: number | null;
  format?: (n: number) => string;
}) {
  const parts = [
    { label: "当前", value: current, tone: 0 },
    {
      label: "历史",
      value: history ?? 0,
      tone: 1,
      missing: history === null,
    },
    { label: "回收站", value: trash, tone: 2 },
    { label: "预留", value: reserved, tone: 3 },
  ];
  const known = parts.reduce((sum, p) => sum + p.value, 0);
  if (quota == null || quota <= 0) {
    return (
      <div className="meter-bar" data-quota="unknown">
        <div className="meter-track" data-filled="false" />
        <small>配额未知</small>
        <ul>
          {parts.map((p) => (
            <li key={p.label} className={"tone-" + p.tone}>
              {p.label} {p.missing ? "未保证保留" : format(p.value)}
            </li>
          ))}
        </ul>
      </div>
    );
  }
  return (
    <div className="meter-bar" data-quota="known">
      <div className="meter-track" data-filled="true">
        {parts.map((p) => (
          <i
            key={p.label}
            className={"tone-" + p.tone}
            style={{ width: `${Math.min(100, (p.value / quota) * 100)}%` }}
          />
        ))}
      </div>
      <small>
        {format(known)} / {format(quota)}
      </small>
    </div>
  );
}
