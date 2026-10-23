import type { DriveFile } from "../../api";
import { NotebookWorkspace } from "./NotebookWorkspace";
import { MarkdownFileDocument } from "./MarkdownFileDocument";

import { canOpenMarkdownNotebook } from "./notebook-capability";
export { canOpenMarkdownNotebook } from "./notebook-capability";
export function MarkdownNotebookPreview({ actor, file, siblings }: { actor: string; file: DriveFile; siblings: DriveFile[] }) {
  const ref = { sourceId: file.space_id, objectId: file.id, revision: file.version_id };
  const files = [file, ...siblings.filter(item => item.id !== file.id)].filter(item => canOpenMarkdownNotebook(item, file.space_id));
  const entries = files.map(item => ({ title: item.path.split("/").pop() || item.path, path: item.path, ref: { sourceId: item.space_id, objectId: item.id, revision: item.version_id } }));
  return <NotebookWorkspace scope={{ actor, space: file.space_id, source: file.space_id }} entries={entries} initialRef={ref} fileRelations
    renderDocument={({ ref: identity, mode, onModeChange, title, path }) => <MarkdownFileDocument identity={identity} mode={mode} onModeChange={onModeChange} title={title} path={path}/>}/>;
}
