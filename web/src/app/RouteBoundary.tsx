import { Component, type ReactNode } from "react";
import { Button } from "../components/ui/button";

// Render failures only. Request failures and mutation recovery remain with pages.
export class RouteBoundary extends Component<{ scope: string; children: ReactNode }, { failed: boolean; scope: string }> {
  state = { failed: false, scope: this.props.scope };
  static getDerivedStateFromError() { return { failed: true }; }
  static getDerivedStateFromProps(props: { scope: string }, state: { scope: string }) {
    return props.scope === state.scope ? null : { failed: false, scope: props.scope };
  }
  render() {
    if (!this.state.failed) return this.props.children;
    return <section role="alert" className="unavailable-drive">
      <h2>此页面暂时无法显示</h2>
      <p>可以切换到其他页面，或重试加载当前页面。未提交的页面输入可能需要重新填写。</p>
      <Button type="button" variant="outline" onClick={() => this.setState({ failed: false })}>重试显示页面</Button>
    </section>;
  }
}
