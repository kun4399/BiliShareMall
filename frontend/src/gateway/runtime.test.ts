import test from 'node:test';
import assert from 'node:assert/strict';
import { hasWailsRuntime, resolveAppRuntime } from './runtime';

test('resolveAppRuntime returns wails when window.go.app.App exists', () => {
  const candidate = {
    go: {
      app: {
        App: {}
      }
    }
  };

  assert.equal(hasWailsRuntime(candidate), true);
  assert.equal(resolveAppRuntime(candidate), 'wails');
});

test('resolveAppRuntime returns web when wails bridge is absent', () => {
  assert.equal(hasWailsRuntime({}), false);
  assert.equal(resolveAppRuntime({}), 'web');
});
