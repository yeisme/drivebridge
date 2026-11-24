import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { parsePastedInviteOrKey } from "./url-token";

interface KeyLoginFormProps {
  value: string;
  busy: boolean;
  secondary: boolean;
  onChange: (value: string) => void;
  onSubmit: (overrideKey?: string) => void;
}

// Controlled by the session shell; never persist or emit the entered key.
// Supports pasting either a raw dbu_ key, a ?token= URL, or a multi-line invite snippet.
export function KeyLoginForm({ value, busy, secondary, onChange, onSubmit }: KeyLoginFormProps) {
  return (
    <form
      className="key-login"
      onSubmit={(event) => {
        event.preventDefault();
        const resolved = parsePastedInviteOrKey(value);
        if (!busy && resolved) {
          if (resolved !== value) onChange(resolved);
          onSubmit(resolved);
        }
      }}
    >
      <label htmlFor="login-key">使用密钥登录</label>
      <Input
        id="login-key"
        type="password"
        value={value}
        autoComplete="off"
        placeholder="粘贴访问密钥 dbu_… 或邀请链接"
        onPaste={(event) => {
          const text = event.clipboardData?.getData("text");
          if (!text) return;
          const parsed = parsePastedInviteOrKey(text);
          if (parsed && parsed !== text.trim()) {
            event.preventDefault();
            onChange(parsed);
          }
        }}
        onChange={(event) => onChange(event.target.value)}
      />
      <small className="text-xs text-muted-foreground">
        支持直接粘贴整段邀请口令或带 <code>?token=</code> 的免密直连链接
      </small>
      <Button variant={secondary ? "outline" : "default"} type="submit" disabled={busy || !value.trim()}>
        {busy ? "验证中…" : "登录"}
      </Button>
    </form>
  );
}
