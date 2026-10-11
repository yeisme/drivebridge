import { Button } from "../components/ui/button";
import { LocalWorkspace } from "../LocalWorkspace";
import { AppearanceProvider, initializeAppearance } from "../appearance";
import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { api, errorText, setCSRF } from "../api";
import { InsightsPanel } from "./Insights";
import "../style.css";
import "../finder.css";
import "../finder-browser.css";
function View() {
  const [ready, setReady] = useState(false),
    [error, setError] = useState("");
  const [tab, setTab] = useState("files"), [csrf, setLocalCSRF] = useState("");
  useEffect(() => {
    (async () => {
      const fragment = new URLSearchParams(location.hash.slice(1));
      const launch = fragment.get("launch");
      if (launch) {
        history.replaceState(null, "", location.pathname + location.search);
        await api("/local-session", "POST", { launch });
      }
      const me = await api<{ csrf_token: string }>("/me");
      setCSRF(me.csrf_token);
      setLocalCSRF(me.csrf_token);
      setReady(true);
    })().catch((e) => setError(errorText(e)));
  }, []);
  return (
    <main className="insight-standalone">
      <header>
        <div>
          <span className="eyebrow">DRIVEBRIDGE · LOCAL INSIGHTS</span>
          <h1>本机文件空间</h1>
        </div>
        <span>已授权空间 · 私有连接</span>
      </header>
      {error && (
        <p role="alert">
          {error}。请通过 drivebridge view 的私有启动文件打开。
        </p>
      )}
      {ready ? <><nav className="local-tabs"><Button variant="outline" type="submit"  aria-pressed={tab==="files"} onClick={()=>setTab("files")}>我的网盘</Button><Button variant="outline" type="submit"  aria-pressed={tab==="insights"} onClick={()=>setTab("insights")}>总览</Button></nav>{tab==="files"?<LocalWorkspace csrf={csrf}/>:<InsightsPanel local />}</> : <p>正在建立本机会话…</p>}
    </main>
  );
}
initializeAppearance();
createRoot(document.getElementById("root")!).render(<AppearanceProvider><View /></AppearanceProvider>);
