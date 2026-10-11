import { frontmatter } from "./properties";
import { StateField, StateEffect, EditorState, Range } from "@codemirror/state";
import {
  Decoration,
  EditorView,
  WidgetType,
  DecorationSet,
} from "@codemirror/view";
import { syntaxTree } from "@codemirror/language";
export const refreshLive = StateEffect.define<boolean>();
type RenderBlock = (
  node: HTMLElement,
  body: string,
  from: number,
  to: number,
) => () => void;
class MarkdownBlock extends WidgetType {
  private dispose?: () => void;
  constructor(
    readonly body: string,
    readonly render: RenderBlock,
    readonly from: number,
    readonly to: number,
  ) {
    super();
  }
  eq(other: MarkdownBlock) {
    return (
      this.body === other.body &&
      this.from === other.from &&
      this.to === other.to
    );
  }
  toDOM(view: EditorView) {
    const node = document.createElement("div");
    node.className = "cursor-text";
    node.dataset.liveBlock = "true";
    this.dispose = this.render(node, this.body, this.from, this.to);
    requestAnimationFrame(() => view.requestMeasure());
    return node;
  }
  updateDOM(node: HTMLElement) {
    this.dispose = this.render(node, this.body, this.from, this.to);
    return true;
  }
  destroy() {
    this.dispose?.();
  }
  ignoreEvent(event: Event) {
    return (
      event.target instanceof Element &&
      !!event.target.closest("button,a,input")
    );
  }
}
export function createLiveFormatting(render: RenderBlock) {
  function decorations(state: EditorState): DecorationSet {
    const ranges: Range<Decoration>[] = [];
    const end = frontmatter(state.doc.toString()).bodyStart;
    if (end)
      ranges.push(
        Decoration.replace({ block: true }).range(0, Math.max(0, end - 1)),
      );
    const tree = syntaxTree(state);
    let node = tree.topNode.firstChild;
    while (node) {
      const active = state.selection.ranges.some(
        (r) => r.from <= node!.to && r.to >= node!.from,
      );
      if (node.from >= end && !active && node.to > node.from) {
        ranges.push(
          Decoration.replace({
            widget: new MarkdownBlock(
              state.doc.sliceString(node.from, node.to),
              render,
              node.from,
              node.to,
            ),
            block: true,
          }).range(node.from, node.to),
        );
      }
      node = node.nextSibling;
    }
    return Decoration.set(ranges, true);
  }
  return StateField.define<{ decorations: DecorationSet; composing: boolean }>({
    create: (state) => ({ decorations: decorations(state), composing: false }),
    update(value, tr) {
      let composing = value.composing;
      for (const effect of tr.effects)
        if (effect.is(refreshLive)) composing = effect.value;
      if (composing)
        return { decorations: value.decorations.map(tr.changes), composing };
      return {
        decorations:
          tr.docChanged ||
          tr.selection ||
          tr.effects.some((e) => e.is(refreshLive))
            ? decorations(tr.state)
            : value.decorations,
        composing,
      };
    },
    provide: (f) => EditorView.decorations.from(f, (v) => v.decorations),
  });
}
