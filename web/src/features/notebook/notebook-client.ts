import { api } from "../../api";
import type { NoteIdentity } from "./notebook-state";
import type { components } from "../../generated/personal-workspace";

export type NotebookCreation = components["schemas"]["NotebookCreationEnvelope"]["data"];
export type DirectoryCreation = components["schemas"]["DirectoryCreationEnvelope"]["data"];
export const createNotebookPage = (spaceId: string, title: string, parentFileId: string, icon: string, key: string, signal: AbortSignal, directoryPath = "") =>
  api<NotebookCreation>("/notebook/pages", "POST", { space_id: spaceId, title, directory_path: directoryPath, parent_file_id: parentFileId, icon, daily_date: "" }, { "Idempotency-Key": key }, signal);
export const createNotebookDaily = (spaceId: string, date: string, key: string, signal: AbortSignal, directoryPath = "") =>
  api<NotebookCreation>("/notebook/pages", "POST", { space_id: spaceId, title: date, directory_path: directoryPath, daily_date: date, parent_file_id: "", icon: "calendar" }, { "Idempotency-Key": key }, signal);
export const lookupNotebookCreation = (spaceId: string, key: string, dailyDate: string, signal: AbortSignal, directoryPath = "") =>
  api<NotebookCreation>(`/notebook/pages/lookup?space_id=${encodeURIComponent(spaceId)}`, "POST", { idempotency_key: key, directory_path: directoryPath, daily_date: dailyDate }, {}, signal);
export const resumeNotebookCreation = (spaceId: string, key: string, dailyDate: string, signal: AbortSignal, directoryPath = "") =>
  api<NotebookCreation>(`/notebook/pages/resume?space_id=${encodeURIComponent(spaceId)}`, "POST", { idempotency_key: key, directory_path: directoryPath, daily_date: dailyDate }, {}, signal);
export const createNotebookDirectory = (spaceId: string, path: string, key: string, signal: AbortSignal) =>
  api<DirectoryCreation>("/directories", "POST", { space_id: spaceId, path }, { "Idempotency-Key": key }, signal);
export const lookupNotebookDirectory = (spaceId: string, key: string, signal: AbortSignal) =>
  api<DirectoryCreation>("/directories/lookup", "POST", { space_id: spaceId, idempotency_key: key }, {}, signal);
export const patchNotebookPage = (ref: NoteIdentity, parentFileId: string, icon: string, expectedRevision: number, key: string, signal: AbortSignal) =>
  api<{ file_id: string; result_revision: number }>(`/notebook/pages/${encodeURIComponent(ref.objectId)}`, "PATCH", { parent_file_id: parentFileId, icon, expected_revision: expectedRevision }, { "Idempotency-Key": key }, signal);
