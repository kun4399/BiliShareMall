export interface MonitorRuleSkuTarget {
  skuId: number | null;
  skuName: string;
  lastLookupSkuId?: number;
}

export function seedMonitorRuleSkuNameCache(
  rules: MonitorRuleSkuTarget[],
  cache: Map<number, string>
) {
  rules.forEach(rule => {
    const skuId = Number(rule.skuId || 0);
    const skuName = rule.skuName.trim();
    if (skuId > 0 && skuName) {
      cache.set(skuId, skuName);
    }
  });
}

export async function hydrateMissingMonitorRuleSkuNames(
  rules: MonitorRuleSkuTarget[],
  lookupSkuName: (skuId: number) => Promise<string>
) {
  await Promise.all(
    rules.map(async rule => {
      const skuId = Number(rule.skuId || 0);
      if (skuId <= 0 || rule.skuName.trim()) {
        return;
      }

      const skuName = (await lookupSkuName(skuId)).trim();
      if (!skuName) {
        return;
      }

      rule.skuName = skuName;
      rule.lastLookupSkuId = skuId;
    })
  );
}
