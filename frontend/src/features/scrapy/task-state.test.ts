import test from 'node:test';
import assert from 'node:assert/strict';
import { applyTaskUiStateTransition, createTaskUiState } from './task-state';

test('failed state is replaced by retrying state after retry event', () => {
  const failed = applyTaskUiStateTransition(undefined, { type: 'failed', at: 100 });
  const retried = applyTaskUiStateTransition(failed, {
    type: 'retry_wait',
    seconds: 12,
    reason: 'request failed: timeout',
    at: 200
  });

  assert.equal(retried.kind, 'retrying');
  assert.equal(retried.retrySeconds, 12);
  assert.equal(retried.retryReason, 'request failed: timeout');
});

test('failed state is cleared once task completes a round', () => {
  const failed = applyTaskUiStateTransition(undefined, { type: 'failed', at: 100 });
  const completed = applyTaskUiStateTransition(failed, { type: 'completed', at: 300 });

  assert.equal(completed.kind, 'completed');
  assert.equal(completed.lastCompletedAt, 300);
  assert.equal(completed.retryReason, '');
});

test('manual start clears stale failure state', () => {
  const failed = applyTaskUiStateTransition(undefined, { type: 'failed', at: 100 });
  const restarted = applyTaskUiStateTransition(failed, { type: 'start', at: 150 });

  assert.equal(restarted.kind, 'starting');
  assert.equal(restarted.retrySeconds, 0);
  assert.equal(restarted.retryReason, '');
});

test('repeated transitions keep a single coherent state payload', () => {
  const firstRetry = applyTaskUiStateTransition(createTaskUiState(), {
    type: 'retry_wait',
    seconds: 8,
    reason: 'network',
    at: 200
  });
  const secondRetry = applyTaskUiStateTransition(firstRetry, {
    type: 'retry_wait',
    seconds: 5,
    reason: 'timeout',
    at: 300
  });

  assert.equal(secondRetry.kind, 'retrying');
  assert.equal(secondRetry.retrySeconds, 5);
  assert.equal(secondRetry.retryReason, 'timeout');
  assert.equal(secondRetry.lastUpdatedAt, 300);
});

test('runtime events from an older run cannot overwrite the current task state', () => {
  const current = applyTaskUiStateTransition(undefined, {
    type: 'runtime',
    payload: {
      taskId: 1,
      runId: 2,
      state: 'running',
      phase: 'requesting',
      retryAt: 0,
      reasonCode: '',
      message: '正在获取商品数据',
      updatedAt: 300,
      lastSuccessAt: 0,
      completedPages: 0,
      completedRounds: 0
    }
  });
  const stale = applyTaskUiStateTransition(current, {
    type: 'runtime',
    payload: {
      taskId: 1,
      runId: 1,
      state: 'failed',
      phase: 'failed',
      retryAt: 0,
      reasonCode: 'request_failed',
      message: '旧任务失败',
      updatedAt: 400,
      lastSuccessAt: 0,
      completedPages: 0,
      completedRounds: 0
    }
  });

  assert.equal(stale.runId, 2);
  assert.equal(stale.kind, 'running');
  assert.equal(stale.message, '正在获取商品数据');
});

test('authoritative stopped runtime state clears retry information', () => {
  const retrying = applyTaskUiStateTransition(undefined, {
    type: 'retry_wait',
    seconds: 60,
    reason: '限流'
  });
  const stopped = applyTaskUiStateTransition(retrying, {
    type: 'runtime',
    payload: {
      taskId: 1,
      runId: 1,
      state: 'stopped',
      phase: 'stopped',
      retryAt: 0,
      reasonCode: '',
      message: '任务已停止',
      updatedAt: 500,
      lastSuccessAt: 0,
      completedPages: 3,
      completedRounds: 1
    }
  });

  assert.equal(stopped.kind, 'idle');
  assert.equal(stopped.retrySeconds, 0);
  assert.equal(stopped.retryReason, '');
});
