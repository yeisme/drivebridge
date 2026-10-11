import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";

interface KeyLoginFormProps {
  value: string;
  busy: boolean;
  secondary: boolean;
  onChange: (value: string) => void;
  onSubmit: () => void;
}

// Controlled by the session shell; never persist or emit the entered key.
export function KeyLoginForm({ value, busy, secondary, onChange, onSubmit }: KeyLoginFormProps) {
  return <form className="key-login" onSubmit={(event) => {
    event.preventDefault();
    if (!busy && value.trim()) onSubmit();
  }}>
    <label htmlFor="login-key">使用密钥登录</label>
    <Input id="login-key" type="password" value={value} autoComplete="off"
      placeholder="dbu_…" onChange={(event) => onChange(event.target.value)} />
    <Button variant={secondary ? "outline" : "default"} type="submit" disabled={busy || !value.trim()}>
      {busy ? "验证中…" : "登录"}
    </Button>
  </form>;
}
