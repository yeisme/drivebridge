export interface WikiReference {
  target: string;
  anchor?: string;
  kind: "note" | "heading" | "block";
  label: string;
  embed: boolean;
}
interface Node {
  type: string;
  value?: string;
  children?: Node[];
  data?: { hName: string; hProperties: Record<string, string> };
}

export function parseWikiReference(value: string, embed = false): WikiReference | undefined {
  if (/[\[\]\r\n]/.test(value)) return;
  const [destination, ...alias] = value.split("|");
  const hash = destination.indexOf("#");
  const target = (hash < 0 ? destination : destination.slice(0, hash)).trim();
  const fragment = hash < 0 ? undefined : destination.slice(hash + 1).trim();
  if ((!target && !fragment) || (hash >= 0 && !fragment)) return;
  const block = fragment?.startsWith("^") ?? false;
  if (block && fragment === "^") return;
  return { target, anchor: block ? fragment!.slice(1) : fragment, kind: fragment ? block ? "block" : "heading" : "note", label: alias.join("|").trim() || destination.trim(), embed };
}

// Transform text nodes only. Code, HTML, images and existing links retain their
// original semantics; this is syntax recognition, never identity resolution.
export function notebookWikiLinks() {
  return (tree: Node) => {
    function visit(node: Node) {
      if (["code", "inlineCode", "html", "link", "linkReference", "image", "imageReference"].includes(node.type) || !node.children) return;
      node.children = node.children.flatMap(child => {
        if (child.type !== "text" || !child.value) { visit(child); return [child]; }
        const result: Node[] = [];
        const pattern = /(?<!\[)(!?)\[\[([^\[\]\r\n]+)\]\](?!\])/g;
        let offset = 0;
        for (const match of child.value.matchAll(pattern)) {
          const ref = parseWikiReference(match[2], match[1] === "!");
          if (!ref) continue;
          if (match.index! > offset) result.push({ type: "text", value: child.value.slice(offset, match.index) });
          result.push({ type: "text", value: ref.label, data: { hName: "span", hProperties: { "data-wiki-reference": JSON.stringify(ref) } } });
          offset = match.index! + match[0].length;
        }
        if (!offset) return [child];
        if (offset < child.value.length) result.push({ type: "text", value: child.value.slice(offset) });
        return result;
      });
    }
    visit(tree);
  };
}
