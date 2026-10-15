import { api } from "../../api";
import type { NoteIdentity } from "./notebook-state";
import type { components } from "../../generated/personal-workspace";

export type NotebookCreation = components["schemas"]["NotebookCreationEnvelope"]["data"];
export const createNotebookPage = (spaceId: string, title: string, parentFileId: string, icon: string, key: string, signal: AbortSignal) =>
  api<NotebookCreation>("/notebook/pages", "POST", { space_id: spaceId, title, parent_file_id: parentFileId, icon, daily_date: "" }, { "Idempotency-Key": key }, signal);
export const createNotebookDaily = (spaceId: string, date: string, key: string, signal: AbortSignal) =>
  api<NotebookCreation>("/notebook/pages", "POST", { space_id: spaceId, title: date, daily_date: date, parent_file_id: "", icon: "calendar" }, { "Idempotency-Key": key }, signal);
export const lookupNotebookCreation = (spaceId: string, key: string, dailyDate: string, signal: AbortSignal) =>
  api<NotebookCreation>(`/notebook/pages/lookup?space_id=${encodeURIComponent(spaceId)}`, "POST", { idempotency_key: key, daily_date: dailyDate }, {}, signal);
export const resumeNotebookCreation = (spaceId: string, key: string, dailyDate: string, signal: AbortSignal) =>
  api<NotebookCreation>(`/notebook/pages/resume?space_id=${encodeURIComponent(spaceId)}`, "POST", { idempotency_key: key, daily_date: dailyDate }, {}, signal);
export const patchNotebookPage = (ref: NoteIdentity, parentFileId: string, icon: string, expectedRevision: number, key: string, signal: AbortSignal) =>
  api<{ file_id: string; result_revision: number }>(`/notebook/pages/${encodeURIComponent(ref.objectId)}`, "PATCH", { parent_file_id: parentFileId, icon, expected_revision: expectedRevision }, { "Idempotency-Key": key }, signal);
