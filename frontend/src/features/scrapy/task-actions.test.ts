import test from 'node:test';
import assert from 'node:assert/strict';
import { beginTaskAction, finishTaskAction } from './task-actions';

test('task action guard rejects a duplicate action until the first one finishes', () => {
  const first = beginTaskAction({}, 7, 'start');
  const duplicate = beginTaskAction(first.actions, 7, 'stop');

  assert.equal(first.accepted, true);
  assert.equal(duplicate.accepted, false);
  assert.equal(duplicate.actions[7], 'start');

  const cleared = finishTaskAction(first.actions, 7);
  const next = beginTaskAction(cleared, 7, 'stop');
  assert.equal(next.accepted, true);
  assert.equal(next.actions[7], 'stop');
});

test('task action guard allows actions for different tasks', () => {
  const first = beginTaskAction({}, 1, 'start');
  const second = beginTaskAction(first.actions, 2, 'stop');

  assert.equal(second.accepted, true);
  assert.deepEqual(second.actions, { 1: 'start', 2: 'stop' });
});
