import { parseDocument, isMap, isScalar, isSeq, stringify } from "yaml";
import { markdownLanguage } from "@codemirror/lang-markdown";

export type PropertyValue = string | number | boolean | null | string[];
export type PropertyType =
  | "text"
  | "list"
  | "number"
  | "checkbox"
  | "date"
  | "datetime";
export interface PropertyDefinition {
  key: string;
  label: string;
  type: PropertyType;
  group: string;
  options?: string[];
}
export interface PropertyField {
  key: string;
  value: PropertyValue;
  complex: boolean;
  from: number;
  to: number;
}
export function frontmatter(source: string) {
  const open = /^(?:\uFEFF)?---\r?\n/.exec(source);
  if (!open) return { start: 0, end: 0, bodyStart: 0, yaml: "" };
  const close = /^(?:---|\.\.\.)[ \t]*(?:\r?\n|$)/m.exec(
    source.slice(open[0].length),
  );
  if (!close)
    return {
      start: open[0].length,
      end: source.length,
      bodyStart: source.length,
      yaml: source.slice(open[0].length),
      unclosed: true,
    };
  const end = open[0].length + close.index;
  return {
    start: open[0].length,
    end,
    bodyStart: end + close[0].length,
    yaml: source.slice(open[0].length, end),
  };
}
export function readProperties(source: string) {
  const region = frontmatter(source);
  const doc = parseDocument(region.yaml, {
    uniqueKeys: true,
    keepSourceTokens: true,
    strict: true,
  });
  const errors = [...doc.errors.map((e) => e.message)];
  if (region.unclosed) errors.unshift("文首 YAML 缺少结束标记 ---");
  if (doc.contents && !isMap(doc.contents))
    errors.push("文首属性必须是键值映射");
  const fields: PropertyField[] = [];
  if (isMap(doc.contents))
    for (const pair of doc.contents.items) {
      if (!isScalar(pair.key) || typeof pair.key.value !== "string") {
        errors.push("属性键必须为文本");
        continue;
      }
      const node = pair.value;
      const simple =
        node == null ||
        (isScalar(node) &&
          !node.anchor &&
          ["string", "number", "boolean", "object"].includes(
            typeof node.value,
          ) &&
          (node.value == null || typeof node.value !== "object")) ||
        (isSeq(node) &&
          !node.anchor &&
          node.items.every(
            (x) => isScalar(x) && !x.anchor && typeof x.value === "string",
          ));
      const value = simple
        ? node == null
          ? null
          : isScalar(node)
            ? (node.value as PropertyValue)
            : (node as any).items.map((x: any) => x.value)
        : null;
      const range = node && "range" in node ? node.range : null;
      fields.push({
        key: pair.key.value,
        value,
        complex: !simple,
        from: region.start + (range?.[0] ?? pair.key.range![1] + 1),
        to: region.start + (range?.[1] ?? pair.key.range![1] + 1),
      });
    }
  return { region, doc, fields, errors, valid: !errors.length };
}
export function patchProperty(
  source: string,
  key: string,
  value: PropertyValue | undefined,
): string {
  if (
    ["__proto__", "constructor", "prototype"].includes(key) ||
    !/^[\p{L}_][\p{L}\p{N}_ /.-]*$/u.test(key)
  )
    throw new Error("字段键格式不支持");
  const p = readProperties(source);
  if (!p.valid) throw new Error(p.errors[0]);
  const f = p.fields.find((f) => f.key === key);
  if (f?.complex) throw new Error("复杂结构请在源码中编辑");
  const eol = source.includes("\r\n") ? "\r\n" : "\n";
  const encoded =
    value === undefined
      ? ""
      : stringify(value, { collectionStyle: "flow", lineWidth: 0 }).trimEnd();
  if (f) {
    if (value === undefined) {
      const pair = (p.doc.contents as any).items.find(
        (x: any) => x.key.value === key,
      );
      const from =
        source.lastIndexOf("\n", p.region.start + pair.key.range[0] - 1) + 1;
      const to = source.indexOf(
        "\n",
        Math.max(
          f.to,
          p.region.start + (pair.value?.range?.[2] ?? pair.key.range[2]),
        ) - 1,
      );
      return (
        source.slice(0, from) + source.slice(to < 0 ? p.region.end : to + 1)
      );
    }
    // Patch the value source range, preserving other keys, comments and the body verbatim.
    const needsSpace = f.from > 0 && source[f.from - 1] === ":" ? " " : "";
    const trailing = /\r?\n$/.test(source.slice(f.from, f.to)) ? eol : "";
    return (
      source.slice(0, f.from) +
      needsSpace +
      encoded +
      trailing +
      source.slice(f.to)
    );
  }
  if (value === undefined) return source;
  const entry = `${stringify(key).trim()}: ${encoded}${eol}`;
  if (!p.region.bodyStart) return `---${eol}${entry}---${eol}${source}`;
  return source.slice(0, p.region.end) + entry + source.slice(p.region.end);
}
export function bodyWithoutProperties(source: string) {
  return source.slice(frontmatter(source).bodyStart);
}
export function typeMatches(value: PropertyValue, type: PropertyType) {
  if (value === null) return true;
  if (type === "list") return Array.isArray(value);
  if (type === "checkbox") return typeof value === "boolean";
  if (type === "number")
    return typeof value === "number" && Number.isFinite(value);
  if (type === "date")
    return (
      typeof value === "string" &&
      /^\d{4}-\d{2}-\d{2}$/.test(value) &&
      !Number.isNaN(Date.parse(value))
    );
  if (type === "datetime")
    return (
      typeof value === "string" &&
      /^\d{4}-\d{2}-\d{2}T/.test(value) &&
      !Number.isNaN(Date.parse(value))
    );
  return typeof value === "string";
}
export function parsePropertyInput(
  value: string,
  type: PropertyType,
): PropertyValue {
  if (type === "list")
    return [
      ...new Set(
        value
          .split(/[,，\n]/)
          .map((x) => x.trim())
          .filter(Boolean),
      ),
    ];
  if (type === "checkbox") return value === "true";
  if (type === "number") {
    if (!value.trim()) return null;
    const n = Number(value);
    if (!Number.isFinite(n)) throw new Error("请输入有效数字");
    return n;
  }
  if (
    (type === "date" || type === "datetime") &&
    value &&
    !typeMatches(value, type)
  )
    throw new Error("日期格式不正确");
  return value;
}
export function displayProperty(value: PropertyValue | undefined) {
  return value === undefined
    ? "未设置"
    : value === null
      ? "空值"
      : Array.isArray(value)
        ? value.length
          ? value.join("、")
          : "空列表"
        : value === ""
          ? "空文本"
          : String(value);
}
export function documentTags(source: string) {
  const p = readProperties(source),
    tags: { name: string; source: "frontmatter" | "inline"; from: number }[] =
      [];
  if (p.valid) {
    const f = p.fields.find((x) => x.key === "tags");
    if (f && !f.complex)
      for (const name of Array.isArray(f.value)
        ? f.value
        : typeof f.value === "string"
          ? [f.value]
          : [])
        tags.push({
          name: name.replace(/^#/, ""),
          source: "frontmatter",
          from: f.from,
        });
  }
  const exclusions: [number, number][] = [[0, p.region.bodyStart]];
  markdownLanguage.parser.parse(source).iterate({
    enter(n) {
      if (
        /^(FencedCode|CodeBlock|InlineCode|Link|Image|Autolink|HTMLBlock|HTMLTag)$/.test(
          n.name,
        )
      ) {
        exclusions.push([n.from, n.to]);
        return false;
      }
    },
  });
  for (const m of source.matchAll(/(^|[\s(])#([\p{L}\p{N}_\/-]+)/gu)) {
    const from = m.index! + m[1].length,
      name = m[2];
    if (
      !/[\p{L}_-]/u.test(name) ||
      name.endsWith("/") ||
      exclusions.some(([a, b]) => from >= a && from < b)
    )
      continue;
    tags.push({ name, source: "inline", from });
  }
  return tags;
}
export const defaultDefinitions: PropertyDefinition[] = [
  { key: "project", label: "项目", type: "text", group: "组织" },
  {
    key: "status",
    label: "状态",
    type: "text",
    group: "组织",
    options: ["草稿", "进行中", "已完成"],
  },
  { key: "kind", label: "文档类型", type: "text", group: "组织" },
  { key: "date", label: "日期", type: "date", group: "时间" },
  { key: "tags", label: "标签", type: "list", group: "组织" },
  { key: "aliases", label: "别名", type: "list", group: "标识" },
  { key: "priority", label: "优先级", type: "number", group: "组织" },
  { key: "reviewed", label: "已审核", type: "checkbox", group: "组织" },
];
