export interface MovedFile {
  id: string;
  from: string;
  to: string;
  version_id: string;
}

export interface MovedFolder {
  from: string;
  to: string;
}

export interface LastMove {
  space_id: string;
  files: MovedFile[];
  folders: MovedFolder[];
}

export function moveCount(move: LastMove | undefined) {
  if (!move) return 0;
  return move.files.length + move.folders.length;
}

export function moveNotice(move: LastMove) {
  return `已移动 ${moveCount(move)} 项`;
}
