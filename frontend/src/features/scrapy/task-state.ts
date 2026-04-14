export type TaskStatusKind = 'idle' | 'running' | 'retrying' | 'failed' | 'completed';

export interface TaskUiState {
  kind: TaskStatusKind;
  lastUpdatedAt: number;
  lastCompletedAt: number;
  retrySeconds: number;
  retryReason: string;
}

export type TaskUiEvent =
  | { type: 'hydrate-running'; at?: number }
  | { type: 'start'; at?: number }
  | { type: 'retry_wait'; seconds?: number; reason?: string; at?: number }
  | { type: 'failed'; at?: number }
  | { type: 'completed'; at?: number }
  | { type: 'stop'; at?: number };

export interface TaskStatusMeta {
  tagType: 'default' | 'success' | 'warning' | 'error' | 'info';
  tagLabel: string;
  text: string;
}

function resolveTimestamp(at?: number) {
  return Number(at || Date.now());
}

export function createTaskUiState(partial: Partial<TaskUiState> = {}): TaskUiState {
  return {
    kind: partial.kind || 'idle',
    lastUpdatedAt: partial.lastUpdatedAt || 0,
    lastCompletedAt: partial.lastCompletedAt || 0,
    retrySeconds: partial.retrySeconds || 0,
    retryReason: partial.retryReason || ''
  };
}

export function applyTaskUiStateTransition(previous: TaskUiState | undefined, event: TaskUiEvent): TaskUiState {
  const current = createTaskUiState(previous);
  const at = resolveTimestamp(event.at);

  switch (event.type) {
    case 'hydrate-running':
    case 'start':
      return {
        kind: 'running',
        lastUpdatedAt: at,
        lastCompletedAt: current.lastCompletedAt,
        retrySeconds: 0,
        retryReason: ''
      };

    case 'retry_wait':
      return {
        kind: 'retrying',
        lastUpdatedAt: at,
        lastCompletedAt: current.lastCompletedAt,
        retrySeconds: Number(event.seconds || 10),
        retryReason: event.reason || '请求失败'
      };

    case 'failed':
      return {
        kind: 'failed',
        lastUpdatedAt: at,
        lastCompletedAt: current.lastCompletedAt,
        retrySeconds: 0,
        retryReason: ''
      };

    case 'completed':
      return {
        kind: 'completed',
        lastUpdatedAt: at,
        lastCompletedAt: at,
        retrySeconds: 0,
        retryReason: ''
      };

    case 'stop':
      return {
        kind: 'idle',
        lastUpdatedAt: at,
        lastCompletedAt: 0,
        retrySeconds: 0,
        retryReason: ''
      };

    default:
      return current;
  }
}

export function createTaskUiStateMap(taskIds: number[]) {
  return taskIds.reduce<Record<number, TaskUiState>>((acc, taskId) => {
    acc[taskId] = applyTaskUiStateTransition(undefined, { type: 'hydrate-running' });
    return acc;
  }, {});
}

function formatTimestamp(timestamp: number) {
  if (timestamp <= 0) {
    return '-';
  }
  return new Date(timestamp).toLocaleString();
}

export function describeTaskUiState(state: TaskUiState): TaskStatusMeta {
  switch (state.kind) {
    case 'running':
      return {
        tagType: 'success',
        tagLabel: '运行中',
        text: '任务状态：正在运行中'
      };

    case 'retrying':
      return {
        tagType: 'warning',
        tagLabel: '重试中',
        text: `${state.retrySeconds} 秒后重试，原因：${state.retryReason || '请求失败'}`
      };

    case 'failed':
      return {
        tagType: 'error',
        tagLabel: '执行失败',
        text: `任务失败，请稍后重试。错误时间：${formatTimestamp(state.lastUpdatedAt)}`
      };

    case 'completed':
      return {
        tagType: 'info',
        tagLabel: '本轮完成',
        text: `时间：${formatTimestamp(state.lastCompletedAt || state.lastUpdatedAt)}`
      };

    default:
      return {
        tagType: 'default',
        tagLabel: '待运行',
        text: '任务状态：待运行'
      };
  }
}
