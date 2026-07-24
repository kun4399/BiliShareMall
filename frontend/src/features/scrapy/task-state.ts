export type TaskStatusKind =
  | 'idle'
  | 'starting'
  | 'queued'
  | 'running'
  | 'retrying'
  | 'stopping'
  | 'failed'
  | 'completed';

export interface TaskRuntimePayload {
  taskId: number;
  runId: number;
  state: string;
  phase: string;
  retryAt: number;
  reasonCode: string;
  message: string;
  updatedAt: number;
  lastSuccessAt: number;
  completedPages: number;
  completedRounds: number;
}

export interface TaskUiState {
  kind: TaskStatusKind;
  runId: number;
  phase: string;
  message: string;
  lastUpdatedAt: number;
  lastCompletedAt: number;
  lastSuccessAt: number;
  retryAt: number;
  retrySeconds: number;
  retryReason: string;
  completedPages: number;
  completedRounds: number;
}

export type TaskUiEvent =
  | { type: 'hydrate-running'; at?: number }
  | { type: 'start'; at?: number }
  | { type: 'runtime'; payload: TaskRuntimePayload }
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

function normalizeRuntimeKind(state: string): TaskStatusKind {
  switch (state) {
    case 'starting':
    case 'queued':
    case 'running':
    case 'retrying':
    case 'stopping':
    case 'failed':
      return state;
    case 'stopped':
      return 'idle';
    default:
      return 'idle';
  }
}

export function createTaskUiState(partial: Partial<TaskUiState> = {}): TaskUiState {
  return {
    kind: partial.kind || 'idle',
    runId: partial.runId || 0,
    phase: partial.phase || '',
    message: partial.message || '',
    lastUpdatedAt: partial.lastUpdatedAt || 0,
    lastCompletedAt: partial.lastCompletedAt || 0,
    lastSuccessAt: partial.lastSuccessAt || 0,
    retryAt: partial.retryAt || 0,
    retrySeconds: partial.retrySeconds || 0,
    retryReason: partial.retryReason || '',
    completedPages: partial.completedPages || 0,
    completedRounds: partial.completedRounds || 0
  };
}

function applyRuntimeState(current: TaskUiState, payload: TaskRuntimePayload, at: number): TaskUiState {
  if (payload.runId < current.runId) {
    return current;
  }
  const retryAt = Number(payload.retryAt || 0);
  return {
    kind: normalizeRuntimeKind(payload.state),
    runId: Number(payload.runId || 0),
    phase: payload.phase || '',
    message: payload.message || '',
    lastUpdatedAt: Number(payload.updatedAt || at),
    lastCompletedAt: current.lastCompletedAt,
    lastSuccessAt: Number(payload.lastSuccessAt || current.lastSuccessAt),
    retryAt,
    retrySeconds: retryAt > 0 ? Math.max(1, Math.ceil((retryAt - Date.now()) / 1000)) : 0,
    retryReason: payload.reasonCode ? payload.message || '' : '',
    completedPages: Number(payload.completedPages || 0),
    completedRounds: Number(payload.completedRounds || 0)
  };
}

export function applyTaskUiStateTransition(previous: TaskUiState | undefined, event: TaskUiEvent): TaskUiState {
  const current = createTaskUiState(previous);
  const at = resolveTimestamp('at' in event ? event.at : undefined);
  if (event.type === 'runtime') {
    return applyRuntimeState(current, event.payload, at);
  }

  switch (event.type) {
    case 'hydrate-running':
    case 'start':
      return {
        ...current,
        kind: event.type === 'start' ? 'starting' : 'running',
        phase: event.type === 'start' ? 'starting' : current.phase,
        message: event.type === 'start' ? '正在启动任务' : current.message,
        lastUpdatedAt: at,
        retryAt: 0,
        retrySeconds: 0,
        retryReason: ''
      };

    case 'retry_wait':
      return {
        ...current,
        kind: 'retrying',
        lastUpdatedAt: at,
        retryAt: at + Number(event.seconds || 10) * 1000,
        retrySeconds: Number(event.seconds || 10),
        retryReason: event.reason || '请求失败',
        message: event.reason || '请求失败'
      };

    case 'failed':
      return {
        ...current,
        kind: 'failed',
        lastUpdatedAt: at,
        retryAt: 0,
        retrySeconds: 0,
        retryReason: ''
      };

    case 'completed':
      return {
        ...current,
        kind: 'completed',
        lastUpdatedAt: at,
        lastCompletedAt: at,
        retryAt: 0,
        retrySeconds: 0,
        retryReason: ''
      };

    case 'stop':
      return {
        ...current,
        kind: 'idle',
        phase: 'stopped',
        message: '任务已停止',
        lastUpdatedAt: at,
        retryAt: 0,
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
    case 'starting':
      return {
        tagType: 'info',
        tagLabel: '启动中',
        text: state.message || '正在启动任务'
      };

    case 'queued':
      return {
        tagType: 'info',
        tagLabel: '排队中',
        text: state.message || '正在等待安全请求时隙'
      };

    case 'running':
      return {
        tagType: 'success',
        tagLabel: '运行中',
        text: state.message || '任务正在运行'
      };

    case 'retrying':
      return {
        tagType: 'warning',
        tagLabel: '等待重试',
        text: state.message || `${state.retrySeconds} 秒后自动重试`
      };

    case 'stopping':
      return {
        tagType: 'warning',
        tagLabel: '停止中',
        text: state.message || '正在安全停止任务'
      };

    case 'failed':
      return {
        tagType: 'error',
        tagLabel: '执行失败',
        text: state.message || `任务失败。错误时间：${formatTimestamp(state.lastUpdatedAt)}`
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
        text: state.message || '任务状态：待运行'
      };
  }
}
