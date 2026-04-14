import test from 'node:test';
import assert from 'node:assert/strict';
import { waitForSharedLoginSession } from './shared-session';

test('waitForSharedLoginSession resolves once shared session becomes available', async () => {
  let attempt = 0;

  const synced = await waitForSharedLoginSession(
    async () => {
      attempt += 1;
      return {
        loggedIn: attempt >= 2,
        updatedAt: attempt
      };
    },
    {
      attempts: 3,
      delayMs: 0
    }
  );

  assert.equal(synced, true);
  assert.equal(attempt, 2);
});

test('waitForSharedLoginSession returns false when shared session never becomes available', async () => {
  let attempt = 0;

  const synced = await waitForSharedLoginSession(
    async () => {
      attempt += 1;
      return {
        loggedIn: false,
        updatedAt: 0
      };
    },
    {
      attempts: 4,
      delayMs: 0
    }
  );

  assert.equal(synced, false);
  assert.equal(attempt, 4);
});
