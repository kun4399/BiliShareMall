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

  assert.equal(restarted.kind, 'running');
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
