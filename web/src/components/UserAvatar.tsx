import { useState } from "react";
import type { UserIdentity } from "../api";
export function UserAvatar({ user }: { user: UserIdentity }) {
  const [failed, setFailed] = useState("");
  return user.avatar_ref && failed !== user.avatar_ref ? (
    <img
      className="size-10 shrink-0 rounded-full object-cover"
      src={`/api/v1/me/avatar?v=${encodeURIComponent(user.avatar_ref)}`}
      alt="我的头像"
      onError={() => setFailed(user.avatar_ref || "")}
    />
  ) : (
    <span className="avatar">
      {(user.display_name || user.name || "我").slice(0, 1)}
    </span>
  );
}
