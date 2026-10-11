export function taskPollDelay(operations: readonly { status: string }[]): number {
  return operations.some(operation => !["completed", "cancelled"].includes(operation.status)) ? 2000 : 30000;
}
