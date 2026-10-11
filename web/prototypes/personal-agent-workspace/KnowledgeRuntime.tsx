import { useEffect } from "react";
import { useDemo } from "./ui";
import { clearEditorSessions } from "./MarkdownEditor";
function SaveNote({ id }: { id: string }) {
  const { state: s, dispatch } = useDemo();
  const n = s.knowledge.notes[id];
  useEffect(() => {
    if (n?.dirty && !n.composing && !s.online && n.status === "dirty") {
      dispatch({ type: "k-save", id });
      return;
    }
    if (
      !n ||
      !n.dirty ||
      n.composing ||
      !s.knowledge.options.autosave ||
      !s.online ||
      !["dirty", "saving", "offline"].includes(n.status)
    )
      return;
    const timer = setTimeout(
      () =>
        dispatch({
          type: n.status === "saving" ? "k-save" : "k-save-start",
          id,
        }),
      n.status === "saving" ? 120 : 800,
    );
    return () => clearTimeout(timer);
  }, [
    id,
    n?.draft,
    n?.status,
    n?.composing,
    n?.dirty,
    s.online,
    s.knowledge.options.autosave,
  ]);
  return null;
}
function Upload({ id }: { id: string }) {
  const { state: s, dispatch } = useDemo();
  const u = s.knowledge.uploads.find((x) => x.id === id);
  useEffect(() => {
    if (u?.status !== "pending") return;
    const timer = setTimeout(
      () => dispatch({ type: "k-upload-finish", id }),
      900,
    );
    return () => clearTimeout(timer);
  }, [id, u?.status]);
  return null;
}
export function KnowledgeRuntime() {
  const { state: s } = useDemo();
  const dirty = Object.values(s.knowledge.notes).some((n) => n.dirty);
  useEffect(() => {
    const before = (e: BeforeUnloadEvent) => {
      if (dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", before);
    return () => window.removeEventListener("beforeunload", before);
  }, [dirty]);
  useEffect(() => {
    if (!Object.keys(s.knowledge.notes).length) clearEditorSessions();
  }, [Object.keys(s.knowledge.notes).length]);
  return (
    <>
      {Object.keys(s.knowledge.notes).map((id) => (
        <SaveNote key={id} id={id} />
      ))}
      {s.knowledge.uploads.map((u) => (
        <Upload key={u.id} id={u.id} />
      ))}
    </>
  );
}
