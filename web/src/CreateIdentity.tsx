import { useState } from "react";
import { api, errorText } from "./api";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import { Badge } from "./components/ui/badge";

export function CreateIdentity({
  kind,
  user,
  close,
}: {
  kind: "user" | "team";
  user: { id: string };
  close: () => void;
}) {
  const [name, setName] = useState(""),
    [step, setStep] = useState(0),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [copied, setCopied] = useState(false),
    [activation, setActivation] = useState<{
      user_id: string;
      activation_code: string;
    }>();
  const labels =
    kind === "user"
      ? ["账号信息", "归属与权限", "确认与交接"]
      : ["团队资料", "管理员", "确认创建"];
  const valid = [...name.trim()].length >= 1 && [...name.trim()].length <= 80;
  return (
    <WorkspaceDialog
      title={kind === "user" ? "新建用户" : "新建团队"}
      close={() => {
        if (!busy) close();
      }}
    >
      {activation ? (
        <div className="grid gap-4">
          <h3 className="text-base font-semibold">账号已创建，等待本人激活</h3>
          <p>
            激活信息只在本次创建结果中展示，24 小时有效。请复制后安全交给本人。
          </p>
          <label>
            用户 ID
            <Input readOnly value={activation.user_id} />
          </label>
          <label>
            激活码
            <Input
              type="password"
              readOnly
              value={activation.activation_code}
            />
          </label>
          <Button
            type="button"
            onClick={async () => {
              try {
                await navigator.clipboard.writeText(
                  `用户 ID：${activation.user_id}\n激活码：${activation.activation_code}`,
                );
                setCopied(true);
                setError("");
              } catch {
                setError("无法使用剪贴板，请重试。");
              }
            }}
          >
            {copied ? "已复制激活信息" : "复制激活信息"}
          </Button>
          <p>管理员不会取得用户登录 Key，用户激活时自行设置。</p>
          <Button type="button" variant="outline" onClick={close}>
            完成并返回用户目录
          </Button>
        </div>
      ) : (
        <form
          className="grid gap-5"
          onSubmit={async (e) => {
            e.preventDefault();
            if (!valid) return;
            if (step < 2) {
              setStep(step + 1);
              return;
            }
            setBusy(true);
            setError("");
            try {
              if (kind === "user")
                setActivation(
                  await api("/governance/provision", "POST", {
                    name: name.trim(),
                  }),
                );
              else {
                await api("/governance/teams", "POST", {
                  name: name.trim(),
                  user_id: user.id,
                });
                close();
                location.hash = "#/teams";
              }
              window.dispatchEvent(new Event("drivebridge-governance-changed"));
            } catch (e) {
              setError(errorText(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          <ol className="flex flex-wrap gap-2" aria-label="创建步骤">
            {labels.map((label, i) => (
              <li key={label} aria-current={step === i ? "step" : undefined}>
                <Badge variant={step === i ? "default" : "secondary"}>
                  {i + 1}. {label}
                </Badge>
              </li>
            ))}
          </ol>
          {step === 0 && (
            <label className="grid gap-2">
              {kind === "user" ? "用户名称" : "团队名称"}
              <Input
                autoFocus
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                aria-invalid={!!name && !valid}
              />
              <small className="text-muted-foreground">
                名称需为 1–80 个字符。
              </small>
            </label>
          )}
          {step === 1 && (
            <div className="space-y-2">
              <h3 className="font-medium">
                {kind === "user" ? "新账号的初始权限" : "团队管理员"}
              </h3>
              <p>
                {kind === "user"
                  ? "创建的是待激活普通账号。激活后再通过团队邀请授予成员身份；加入团队不会自动获得文件编辑权限。"
                  : "你将成为团队管理员。创建后进入团队页添加成员并设置团队网盘。"}
              </p>
              <Badge variant="secondary">
                {kind === "user"
                  ? "等待本人激活 · 尚未加入团队"
                  : "当前账号管理团队"}
              </Badge>
            </div>
          )}
          {step === 2 && (
            <div className="space-y-2">
              <h3 className="font-medium">确认创建「{name.trim()}」</h3>
              <p>
                {kind === "user"
                  ? "提交后显示一次激活信息。关闭页面后不能重新取得此激活码。"
                  : "创建团队后可继续配置空间和协作权限。"}
              </p>
            </div>
          )}
          <div className="flex justify-end gap-2">
            {step > 0 && (
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={() => setStep(step - 1)}
              >
                上一步
              </Button>
            )}
            <Button type="submit" disabled={busy || !valid}>
              {busy ? "正在创建…" : step === 2 ? "确认创建" : "下一步"}
            </Button>
          </div>
        </form>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </WorkspaceDialog>
  );
}
