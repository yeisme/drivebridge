import type { DriveFile } from "../../api";

// Keep entry eligibility independent of the lazy reader/editor dependency graph.
export function canOpenMarkdownNotebook(file: DriveFile, activeSpace: string): boolean {
  return Boolean(activeSpace && file.space_id === activeSpace && file.version_id && !file.trashed && !file.preview_reason && !file.preview_url &&
    (/\.(md|markdown)$/i.test(file.path) || file.mime_type === "text/markdown"));
}
