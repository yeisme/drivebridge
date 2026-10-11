import { Component, type ReactNode } from "react";
import { Button } from "../../components/ui/button";

// Render failures are local to one document. Do not log source or exception
// payloads; request failures and write recovery belong to their owner adapter.
export class NotebookRenderBoundary extends Component<{ body: string; children: ReactNode }, { failed: boolean; body: string }> {
  state = { failed: false, body: this.props.body };
  static getDerivedStateFromError() { return { failed: true }; }
  static getDerivedStateFromProps(props: { body: string }, state: { body: string }) {
    return props.body === state.body ? null : { failed: false, body: props.body };
  }
  render() {
    if (!this.state.failed) return this.props.children;
    return <section aria-label="渲染恢复" className="space-y-3">
      <p role="alert">正文渲染失败，原始 Markdown 已保留。</p>
      <Button type="button" variant="outline" onClick={() => this.setState({ failed: false })}>重试渲染</Button>
      <pre aria-label="保留的 Markdown 源码" className="overflow-auto whitespace-pre-wrap break-words rounded border p-3">{this.props.body}</pre>
    </section>;
  }
}
