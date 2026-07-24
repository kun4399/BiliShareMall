export type TaskActionKind = 'start' | 'stop' | 'delete' | 'config';
export type TaskActionMap = Record<number, TaskActionKind>;

export function beginTaskAction(current: TaskActionMap, taskId: number, action: TaskActionKind) {
  if (current[taskId]) {
    return { accepted: false, actions: current };
  }
  return {
    accepted: true,
    actions: {
      ...current,
      [taskId]: action
    }
  };
}

export function finishTaskAction(current: TaskActionMap, taskId: number) {
  if (!current[taskId]) return current;
  return Object.fromEntries(
    Object.entries(current).filter(([currentTaskId]) => Number(currentTaskId) !== taskId)
  ) as TaskActionMap;
}
