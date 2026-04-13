<script setup lang="ts">
import { useClipboard } from '@vueuse/core';
import { onMounted, onUnmounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import { scrapy } from '~/wailsjs/go/models';
import { EventsOn } from '~/wailsjs/runtime/runtime';
import { GetC2CItemNameBySku, GetMonitorConfig, ListMonitorRuleHits, SaveMonitorConfig } from '~/wailsjs/go/app/App';

interface MonitorRuleForm {
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
}

type RuleHitsMap = Record<number, scrapy.MonitorHitItem[]>;

const hitLimitPerRule = 20;
const message = useMessage();
const loadingBar = useLoadingBar();
const { copy, isSupported } = useClipboard();

const webhook = ref('');
const rules = ref<MonitorRuleForm[]>([]);
const ruleHitsMap = ref<RuleHitsMap>({});
const skuLookupTimerMap = new Map<number, ReturnType<typeof setTimeout>>();
const skuNameCache = new Map<number, string>();
const skuLookupPromiseMap = new Map<number, Promise<string>>();
const unlisteners: Array<() => void> = [];
const skuLookupTimeoutMs = 2000;
let ruleKeySeed = 1;

function createRuleForm(partial?: Partial<MonitorRuleForm>): MonitorRuleForm {
  return {
    key: ruleKeySeed++,
    id: partial?.id ?? 0,
    skuId: partial?.skuId ?? null,
    minPriceYuan: partial?.minPriceYuan ?? null,
    maxPriceYuan: partial?.maxPriceYuan ?? null,
    enabled: partial?.enabled ?? true,
    skuName: '',
    skuNameLoading: false,
    skuLookupSeq: 0,
    lastLookupSkuId: 0
  };
}

function normalizeHit(payload: unknown) {
  const hit = scrapy.MonitorHitItem.createFrom(payload);
  if (!hit.itemName) {
    hit.itemName = hit.c2cItemsId ? `商品 #${hit.c2cItemsId}` : '未知商品';
  }
  if (!hit.showPrice && Number.isFinite(hit.price)) {
    hit.showPrice = toYuan(Number(hit.price)).toFixed(2);
  }
  return hit;
}

function upsertRuleHit(hit: scrapy.MonitorHitItem) {
  const ruleId = Number(hit.ruleId || 0);
  if (ruleId <= 0) {
    return;
  }
  const before = ruleHitsMap.value[ruleId] || [];
  const duplicate = before.some(
    item =>
      Number(item.c2cItemsId || 0) === Number(hit.c2cItemsId || 0) &&
      String(item.status || '') === String(hit.status || '') &&
      Number(item.occurredAt || 0) === Number(hit.occurredAt || 0)
  );
  if (duplicate) {
    return;
  }
  const next = [hit, ...before].slice(0, hitLimitPerRule);
  ruleHitsMap.value = {
    ...ruleHitsMap.value,
    [ruleId]: next
  };
}

function getRuleHits(rule: MonitorRuleForm) {
  if (!rule.id || rule.id <= 0) {
    return [];
  }
  return ruleHitsMap.value[rule.id] || [];
}

function formatHitTime(timestamp: number) {
  const value = Number(timestamp || 0);
  if (value <= 0) {
    return '-';
  }
  return new Date(value).toLocaleString();
}

function formatHitPrice(hit: scrapy.MonitorHitItem) {
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

async function copyHitLink(link: string) {
  const normalized = (link || '').trim();
  if (!normalized) {
    message.warning('链接为空');
    return;
  }
  if (!isSupported.value) {
    message.error(`复制失败，请自行复制：${normalized}`);
    return;
  }
  try {
    await copy(normalized);
    message.success('链接已复制');
  } catch (err) {
    message.error(`复制失败，请自行复制：${normalized}`);
  }
}

function hitKey(hit: scrapy.MonitorHitItem, index: number) {
  return `${hit.ruleId}-${hit.c2cItemsId}-${hit.status}-${hit.occurredAt}-${index}`;
}

function addRule() {
  rules.value.push(createRuleForm());
}

function clearSkuLookupTimer(key: number) {
  const timer = skuLookupTimerMap.get(key);
  if (timer) {
    clearTimeout(timer);
    skuLookupTimerMap.delete(key);
  }
}

async function lookupSkuName(rule: MonitorRuleForm) {
  const skuId = Number(rule.skuId ?? 0);
  if (skuId <= 0) {
    rule.skuName = '';
    rule.skuNameLoading = false;
    rule.lastLookupSkuId = 0;
    return;
  }
  if (rule.lastLookupSkuId === skuId && rule.skuName) {
    return;
  }
  const cachedName = skuNameCache.get(skuId);
  if (cachedName !== undefined) {
    rule.skuName = cachedName;
    rule.skuNameLoading = false;
    rule.lastLookupSkuId = skuId;
    return;
  }

  rule.skuNameLoading = true;
  rule.skuLookupSeq += 1;
  const currentSeq = rule.skuLookupSeq;

  try {
    const name = await lookupSkuNameFast(skuId);
    if (rule.skuLookupSeq !== currentSeq) {
      return;
    }
    rule.skuName = name;
    rule.lastLookupSkuId = skuId;
    skuNameCache.set(skuId, name);
  } catch (err) {
    if (rule.skuLookupSeq !== currentSeq) {
      return;
    }
    rule.skuName = '';
  } finally {
    if (rule.skuLookupSeq === currentSeq) {
      rule.skuNameLoading = false;
    }
  }
}

function queueLookupSkuName(rule: MonitorRuleForm) {
  clearSkuLookupTimer(rule.key);
  const timer = setTimeout(() => {
    skuLookupTimerMap.delete(rule.key);
    void lookupSkuName(rule);
  }, 120);
  skuLookupTimerMap.set(rule.key, timer);
}

function getSkuNameDisplayText(rule: MonitorRuleForm) {
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

async function lookupSkuNameFast(skuId: number): Promise<string> {
  const inFlight = skuLookupPromiseMap.get(skuId);
  if (inFlight) {
    return inFlight;
  }

  const timeoutPromise = new Promise<string>(resolve => {
    setTimeout(() => resolve(''), skuLookupTimeoutMs);
  });
  const requestPromise = (async () => {
    try {
      const name = (await GetC2CItemNameBySku(skuId)).trim();
      return name;
    } catch (err) {
      return '';
    }
  })();

  const wrapped = Promise.race([requestPromise, timeoutPromise]).finally(() => {
    skuLookupPromiseMap.delete(skuId);
  });
  skuLookupPromiseMap.set(skuId, wrapped);
  return wrapped;
}

function removeRule(index: number) {
  const rule = rules.value[index];
  if (rule) {
    clearSkuLookupTimer(rule.key);
    if (rule.id > 0) {
      const next = { ...ruleHitsMap.value };
      delete next[rule.id];
      ruleHitsMap.value = next;
    }
  }
  rules.value.splice(index, 1);
}

function toYuan(cents: number) {
  return Number((cents / 100).toFixed(2));
}

function toCents(yuan: number) {
  return Math.round(yuan * 100);
}

function validateRules(items: MonitorRuleForm[]) {
  for (let i = 0; i < items.length; i += 1) {
    const rule = items[i];
    if (!rule.skuId || rule.skuId <= 0) {
      return `第 ${i + 1} 条规则的 skuId 不合法`;
    }
    if (rule.minPriceYuan == null || rule.maxPriceYuan == null) {
      return `第 ${i + 1} 条规则需要填写完整价格区间`;
    }
    if (rule.minPriceYuan < 0 || rule.maxPriceYuan < 0) {
      return `第 ${i + 1} 条规则价格不能小于 0`;
    }
    if (rule.minPriceYuan > rule.maxPriceYuan) {
      return `第 ${i + 1} 条规则最小价格不能大于最大价格`;
    }
  }
  return '';
}

async function loadRuleHits() {
  const groups = await ListMonitorRuleHits(hitLimitPerRule);
  const next: RuleHitsMap = {};
  (groups || []).forEach(group => {
    const ruleId = Number(group.ruleId || 0);
    if (ruleId <= 0) {
      return;
    }
    next[ruleId] = (group.hits || []).map(hit => normalizeHit(hit));
  });
  ruleHitsMap.value = next;
}

async function loadConfig() {
  loadingBar.start();
  try {
    const config = await GetMonitorConfig();
    webhook.value = config.webhook || '';
    const loadedRules = (config.rules || []).map(rule =>
      createRuleForm({
        id: rule.id || 0,
        skuId: rule.skuId || null,
        minPriceYuan: toYuan(rule.minPrice || 0),
        maxPriceYuan: toYuan(rule.maxPrice || 0),
        enabled: rule.enabled ?? true
      })
    );
    rules.value = loadedRules;
    loadedRules.forEach(rule => {
      void lookupSkuName(rule);
    });
    await loadRuleHits();
  } catch (err: any) {
    message.error(err?.message || '读取配置失败');
  } finally {
    loadingBar.finish();
  }
}

async function saveConfig() {
  const cleanedWebhook = webhook.value.trim();
  const validationError = validateRules(rules.value);
  if (validationError) {
    message.warning(validationError);
    return;
  }
  if (rules.value.length > 0 && !cleanedWebhook) {
    message.warning('配置了监控规则时必须填写钉钉 webhook');
    return;
  }

  const payload = scrapy.MonitorConfig.createFrom({
    webhook: cleanedWebhook,
    rules: rules.value.map(rule =>
      scrapy.MonitorRule.createFrom({
        id: rule.id,
        skuId: Number(rule.skuId),
        minPrice: toCents(Number(rule.minPriceYuan)),
        maxPrice: toCents(Number(rule.maxPriceYuan)),
        enabled: rule.enabled
      })
    )
  });

  loadingBar.start();
  try {
    await SaveMonitorConfig(payload);
    message.success('保存成功');
    await loadConfig();
  } catch (err: any) {
    loadingBar.error();
    message.error(err?.message || '保存失败');
  } finally {
    loadingBar.finish();
  }
}

onMounted(() => {
  unlisteners.push(
    EventsOn('monitor_alert_result', payload => {
      upsertRuleHit(normalizeHit(payload));
    })
  );
  void loadConfig();
});

onUnmounted(() => {
  skuLookupTimerMap.forEach(timer => {
    clearTimeout(timer);
  });
  skuLookupTimerMap.clear();
  while (unlisteners.length > 0) {
    const unlisten = unlisteners.pop();
    if (unlisten) {
      unlisten();
    }
  }
});
</script>

<template>
  <NSpace vertical size="medium">
    <NCard title="钉钉 Webhook">
      <NSpace vertical size="small">
        <NInput
          v-model:value="webhook"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 2 }"
          placeholder="填写钉钉机器人 webhook（有规则时必填）"
        />
      </NSpace>
    </NCard>

    <NCard title="监控规则">
      <template #header-extra>
        <NSpace>
          <NButton @click="addRule">
            <template #icon>
              <icon-ic-round-plus />
            </template>
            新增规则
          </NButton>
          <NButton type="primary" @click="saveConfig">保存配置</NButton>
        </NSpace>
      </template>

      <NEmpty v-if="rules.length === 0" description="暂无规则，点击“新增规则”开始配置" />
      <NSpace v-else vertical size="small">
        <NCard
          v-for="(rule, idx) in rules"
          :key="rule.key"
          size="small"
          :title="`规则 #${idx + 1}`"
        >
          <template #header-extra>
            <NSpace align="center">
              <NText depth="3">启用</NText>
              <NSwitch v-model:value="rule.enabled" />
              <NButton quaternary type="error" @click="removeRule(idx)">
                删除
              </NButton>
            </NSpace>
          </template>
          <NGrid :cols="4" :x-gap="8" :y-gap="8">
            <NFormItemGi label="SKU ID">
              <NInputNumber
                v-model:value="rule.skuId"
                :min="1"
                :precision="0"
                placeholder="如 123456"
                @update:value="() => queueLookupSkuName(rule)"
                @blur="() => lookupSkuName(rule)"
              />
            </NFormItemGi>
            <NFormItemGi label="最低价（元）">
              <NInputNumber v-model:value="rule.minPriceYuan" :min="0" :precision="2" placeholder="如 99.00" />
            </NFormItemGi>
            <NFormItemGi label="最高价（元）">
              <NInputNumber v-model:value="rule.maxPriceYuan" :min="0" :precision="2" placeholder="如 199.00" />
            </NFormItemGi>
            <NFormItemGi label="商品名">
              <NText :depth="rule.skuName ? 2 : 3">{{ getSkuNameDisplayText(rule) }}</NText>
            </NFormItemGi>
          </NGrid>

          <NDivider />

          <NSpace vertical size="small">
            <NText depth="3">命中记录（最近 {{ hitLimitPerRule }} 条）</NText>
            <NEmpty v-if="rule.id <= 0" size="small" description="保存规则后开始记录命中结果" />
            <NEmpty v-else-if="getRuleHits(rule).length === 0" size="small" description="暂无命中记录" />
            <NList v-else bordered size="small">
              <NListItem v-for="(hit, hitIdx) in getRuleHits(rule)" :key="hitKey(hit, hitIdx)">
                <NSpace align="center" :size="6" :wrap="false" style="width: 100%">
                  <NText depth="3">{{ formatHitPrice(hit) }} 元</NText>
                  <NText depth="3">｜</NText>
                  <NText depth="3">{{ formatHitTime(hit.occurredAt) }}</NText>
                  <NText depth="3">｜</NText>
                  <NButton text type="primary" @click="() => copyHitLink(hit.itemLink)">链接</NButton>
                </NSpace>
              </NListItem>
            </NList>
          </NSpace>
        </NCard>
      </NSpace>
    </NCard>
  </NSpace>
</template>
