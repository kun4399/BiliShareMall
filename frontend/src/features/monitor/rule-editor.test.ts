import test from 'node:test';
import assert from 'node:assert/strict';
import { buildMonitorConfigPayload, createRuleForm, summarizeRuleHits } from './rule-editor';

test('buildMonitorConfigPayload preserves existing monitor rule fields', () => {
  const payload = buildMonitorConfigPayload('  https://example.com/hook  ', [
    createRuleForm(1, {
      id: 7,
      skuId: 123,
      skuName: '测试商品',
      minPriceYuan: 99.5,
      maxPriceYuan: 199.5,
      enabled: true,
      remark: 'keep-me'
    })
  ]);

  assert.equal(payload.webhook, 'https://example.com/hook');
  assert.equal(payload.rules[0].id, 7);
  assert.equal(payload.rules[0].skuId, 123);
  assert.equal(payload.rules[0].skuName, '测试商品');
  assert.equal(payload.rules[0].minPrice, 9950);
  assert.equal(payload.rules[0].maxPrice, 19950);
  assert.equal(payload.rules[0].remark, 'keep-me');
});

test('summarizeRuleHits returns empty summary when there are no hits', () => {
  const summary = summarizeRuleHits([], 20);

  assert.equal(summary.count, 0);
  assert.equal(summary.summaryText, '暂无命中记录');
});

test('summarizeRuleHits caps label by limit and exposes latest hit time', () => {
  const summary = summarizeRuleHits(
    [
      { occurredAt: 1000 } as any,
      { occurredAt: 2500 } as any,
      { occurredAt: 1500 } as any
    ],
    2
  );

  assert.equal(summary.count, 3);
  assert.match(summary.summaryText, /^最近 2 条，最近一次：/);
  assert.notEqual(summary.latestHitTime, '');
});
