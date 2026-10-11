import type { DriveFile } from "../../api";

// Revalidate selection against the current loaded scope before a restore write.
export function restorableSelection(files: readonly DriveFile[], picked: Record<string, boolean>, space: string): string[] {
  return [...new Set(files.filter(file => file.space_id === space && file.trashed && picked[file.id]).map(file => file.id))];
}
