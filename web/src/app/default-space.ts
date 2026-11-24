import type { Space } from "../api";

export function chooseDefaultDriveSpace(
  spaces: readonly Space[],
  preferredId = "",
): Space | undefined {
  if (!spaces.length) return undefined;
  if (preferredId) {
    const exact = spaces.find((space) => space.id === preferredId);
    if (exact) return exact;
  }
  return spaces.find((space) => !space.team_id) ?? spaces[0];
}
