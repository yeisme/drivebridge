import { useEffect, useId, useRef, useState } from "react";
import { Button } from "../../components/ui/button";

export function NotebookDiagram({ source }: { source: string }) {
  const id = useId().replace(/[^a-zA-Z0-9]/g, "");
  const host = useRef<HTMLDivElement>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    const target = host.current;
    target?.replaceChildren(); setState("loading");
    // Reject configuration, HTML and user CSS/resource constructs before
    // Mermaid's temporary layout DOM is created, not only after SVG output.
    if (source.length > 32000 || /%%\s*\{|^\s*---(?:\r?\n|$)|<|\\|\b(?:classDef|style|linkStyle|click|href|image|img|icon)\b|url\s*\(|@import|https?:|data:|javascript:/im.test(source)) { setState("error"); return; }
    void import("mermaid").then(async ({ default: mermaid }) => {
      if (!active) return;
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", suppressErrorRendering: true, maxTextSize: 32000, maxEdges: 500, htmlLabels: false, theme: "neutral" });
      const result = await mermaid.render(`notebook-diagram-${id}-${attempt}`, source);
      if (!active || !target) return;
      const parsed = new DOMParser().parseFromString(result.svg, "image/svg+xml");
      const svg = parsed.documentElement;
      if (svg.localName !== "svg" || svg.namespaceURI !== "http://www.w3.org/2000/svg" || parsed.querySelector("parsererror, script, foreignObject, image, iframe, object, embed, animate, set, animateTransform, animateMotion")) throw new Error("unsafe_diagram");
      // SVG links must stay within this diagram; strip all executable handlers
      // and external resource references before importing the tree.
      for (const node of [svg, ...Array.from(svg.querySelectorAll("*"))]) {
        for (const attribute of Array.from(node.attributes)) {
          if (/@import|url\(\s*["']?(?!#|["']?#)/i.test(attribute.value)) throw new Error("external_diagram_resource");
          if (/^on/i.test(attribute.name) || (/href$/i.test(attribute.name) && !attribute.value.startsWith("#"))) node.removeAttribute(attribute.name);
        }
      }
      if (/@import|url\(\s*["']?(?!#|["']?#)/i.test(svg.textContent ?? "")) throw new Error("external_diagram_resource");
      target.replaceChildren(document.importNode(svg, true)); setState("ready");
    }).catch(() => { if (active) setState("error"); });
    return () => { active = false; target?.replaceChildren(); };
  }, [source, id, attempt]);
  return <figure className="my-5 min-w-0">
    <div ref={host} role="img" aria-label="笔记图表" hidden={state !== "ready"} className="max-w-full overflow-x-auto rounded-lg bg-card p-3 text-foreground dark:invert dark:hue-rotate-180 [&_svg]:max-w-full"/>
    {state === "loading" && <p role="status" className="text-sm text-muted-foreground">正在渲染图表…</p>}
    {state === "error" && <div role="alert" className="space-y-2"><p className="text-sm">图表无法安全渲染，源码已保留。</p><Button type="button" variant="outline" onClick={() => setAttempt(value => value + 1)}>重试图表</Button></div>}
    <details open={state === "error"} className="text-xs text-muted-foreground"><summary className="min-h-8 cursor-pointer py-1">图表源码</summary><pre className="overflow-x-auto">{source}</pre></details>
  </figure>;
}
