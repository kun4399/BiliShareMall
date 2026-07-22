import test from 'node:test';
import assert from 'node:assert/strict';
import { normalizeImage, resolveReferencePriceLabel } from './shared';

test('normalizeImage lets the browser load CDN images directly', () => {
  assert.equal(normalizeImage('//i0.hdslb.com/test.png'), 'https://i0.hdslb.com/test.png');
  assert.equal(normalizeImage('https://example.com/test.png'), 'https://example.com/test.png');
});

test('resolveReferencePriceLabel prefers backend label when present', () => {
  assert.equal(resolveReferencePriceLabel('参考价 109.00 元', 0, 0), '参考价 109.00 元');
});

test('resolveReferencePriceLabel derives single price fallback from min/max', () => {
  assert.equal(resolveReferencePriceLabel('', 10900, 10900), '参考价 109.00 元');
  assert.equal(resolveReferencePriceLabel('', 0, 12900), '参考价 129.00 元');
});

test('resolveReferencePriceLabel derives range fallback from min/max', () => {
  assert.equal(resolveReferencePriceLabel('', 9900, 12900), '参考价 99.00 - 129.00 元');
});

test('resolveReferencePriceLabel falls back to missing message when reference price is absent', () => {
  assert.equal(resolveReferencePriceLabel('', 0, 0), '参考价待补充');
});
