import { scrapy } from '~/wailsjs/go/models';

export interface MonitorRuleForm {
  key: number;
  id: number;
  skuId: number | null;
  minPriceYuan: number | null;
  maxPriceYuan: number | null;
  enabled: boolean;
  skuName: string;
  skuNameLoading: boolean;
  skuLookupSeq: number;
  lastLookupSkuId: number;
  remark: string;
}

export interface MonitorRuleHitsSummary {
  count: number;
  latestHitTime: string;
  summaryText: string;
}

export function createRuleForm(key: number, partial: Partial<MonitorRuleForm> = {}): MonitorRuleForm {
  const initialSkuId = partial.skuId ?? null;
  const initialSkuName = partial.skuName ?? '';

  return {
    key,
    id: partial.id ?? 0,
    skuId: initialSkuId,
    minPriceYuan: partial.minPriceYuan ?? null,
    maxPriceYuan: partial.maxPriceYuan ?? null,
    enabled: partial.enabled ?? true,
    skuName: initialSkuName,
    skuNameLoading: false,
    skuLookupSeq: 0,
    lastLookupSkuId: initialSkuId && initialSkuName ? Number(initialSkuId) : 0,
    remark: partial.remark ?? ''
  };
}

export function toYuan(cents: number) {
  return Number((cents / 100).toFixed(2));
}

export function toCents(yuan: number) {
  return Math.round(yuan * 100);
}

export function normalizeMonitorHit(payload: unknown) {
  const hit = scrapy.MonitorHitItem.createFrom(payload);
  if (!hit.itemName) {
    hit.itemName = hit.c2cItemsId ? `商品 #${hit.c2cItemsId}` : '未知商品';
  }
  if (!hit.showPrice && Number.isFinite(hit.price)) {
    hit.showPrice = toYuan(Number(hit.price)).toFixed(2);
  }
  return hit;
}

export function formatMonitorHitTime(timestamp: number) {
  const value = Number(timestamp || 0);
  if (value <= 0) {
    return '-';
  }
  return new Date(value).toLocaleString();
}

export function formatMonitorHitPrice(hit: scrapy.MonitorHitItem) {
  const showPrice = (hit.showPrice || '').trim();
  if (showPrice) {
    return showPrice;
  }
  const price = Number(hit.price || 0);
  if (price < 0) {
    return '-';
  }
  return toYuan(price).toFixed(2);
}

export function getSkuNameDisplayText(rule: MonitorRuleForm) {
  if (rule.skuNameLoading) {
    return '查询中...';
  }
  if (!rule.skuId || rule.skuId <= 0) {
    return '输入 SKU 后自动显示';
  }
  if (rule.skuName) {
    return rule.skuName;
  }
  return '未找到商品名（可能尚未入库）';
}

export function summarizeRuleHits(hits: scrapy.MonitorHitItem[], limit: number): MonitorRuleHitsSummary {
  const count = hits.length;
  if (count === 0) {
    return {
      count: 0,
      latestHitTime: '',
      summaryText: '暂无命中记录'
    };
  }

  const latestOccurredAt = Math.max(...hits.map(hit => Number(hit.occurredAt || 0)));
  const latestHitTime = formatMonitorHitTime(latestOccurredAt);

  return {
    count,
    latestHitTime,
    summaryText: `最近 ${Math.min(count, limit)} 条，最近一次：${latestHitTime}`
  };
}

export function buildMonitorConfigPayload(webhook: string, rules: MonitorRuleForm[]) {
  return scrapy.MonitorConfig.createFrom({
    webhook: webhook.trim(),
    rules: rules.map(rule =>
      scrapy.MonitorRule.createFrom({
        id: rule.id,
        skuId: Number(rule.skuId),
        skuName: rule.skuName,
        minPrice: toCents(Number(rule.minPriceYuan)),
        maxPrice: toCents(Number(rule.maxPriceYuan)),
        enabled: rule.enabled,
        remark: rule.remark
      })
    )
  });
}
