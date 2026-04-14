import test from 'node:test';
import assert from 'node:assert/strict';
import type { MonitorRuleSkuTarget } from './sku-name';
import { hydrateMissingMonitorRuleSkuNames, seedMonitorRuleSkuNameCache } from './sku-name';

test('seedMonitorRuleSkuNameCache records resolved names from backend payload', () => {
  const cache = new Map<number, string>();

  seedMonitorRuleSkuNameCache(
    [
      { skuId: 1001, skuName: '测试商品' },
      { skuId: 1002, skuName: '' }
    ],
    cache
  );

  assert.equal(cache.get(1001), '测试商品');
  assert.equal(cache.has(1002), false);
});

test('hydrateMissingMonitorRuleSkuNames backfills only missing rule names', async () => {
  const rules: MonitorRuleSkuTarget[] = [
    { skuId: 1001, skuName: '' },
    { skuId: 1002, skuName: '已有商品名' },
    { skuId: null, skuName: '' }
  ];
  const lookups: number[] = [];

  await hydrateMissingMonitorRuleSkuNames(rules, async skuId => {
    lookups.push(skuId);
    return skuId === 1001 ? '回填商品名' : '';
  });

  assert.deepEqual(lookups, [1001]);
  assert.equal(rules[0].skuName, '回填商品名');
  assert.equal(rules[0].lastLookupSkuId, 1001);
  assert.equal(rules[1].skuName, '已有商品名');
});
