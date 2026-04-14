<script setup lang="ts">
import { useClipboard } from '@vueuse/core';
import { onActivated, onMounted, onUnmounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import { scrapy } from '~/wailsjs/go/models';
import { GetC2CItemNameBySku, GetMonitorConfig, ListMonitorRuleHits, OnAppEvent, SaveMonitorConfig } from '@/gateway';
import { hydrateMissingMonitorRuleSkuNames, seedMonitorRuleSkuNameCache } from '@/features/monitor/sku-name';
import {
  buildMonitorConfigPayload,
  createRuleForm,
  normalizeMonitorHit,
  toYuan,
  type MonitorRuleForm
} from '@/features/monitor/rule-editor';
import MonitorRuleCard from './modules/monitor-rule-card.vue';

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
let ruleKeySeed = 1;
let hasActivatedOnce = false;

function nextRuleKey() {
  const current = ruleKeySeed;
  ruleKeySeed += 1;
  return current;
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

function addRule() {
  rules.value.push(createRuleForm(nextRuleKey()));
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
    if (name) {
      skuNameCache.set(skuId, name);
    }
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

async function lookupSkuNameFast(skuId: number): Promise<string> {
  const inFlight = skuLookupPromiseMap.get(skuId);
  if (inFlight) {
    return inFlight;
  }

  const wrapped = (async () => {
    try {
      return (await GetC2CItemNameBySku(skuId)).trim();
    } catch (err) {
      return '';
    }
  })().finally(() => {
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
    next[ruleId] = (group.hits || []).map(hit => normalizeMonitorHit(hit));
  });
  ruleHitsMap.value = next;
}

async function loadConfig() {
  loadingBar.start();
  try {
    const config = await GetMonitorConfig();
    webhook.value = config.webhook || '';
    const loadedRules = (config.rules || []).map(rule =>
      createRuleForm(nextRuleKey(), {
        id: rule.id || 0,
        skuId: rule.skuId || null,
        skuName: rule.skuName || '',
        minPriceYuan: toYuan(rule.minPrice || 0),
        maxPriceYuan: toYuan(rule.maxPrice || 0),
        enabled: rule.enabled ?? true,
        remark: rule.remark || ''
      })
    );
    seedMonitorRuleSkuNameCache(loadedRules, skuNameCache);
    await hydrateMissingMonitorRuleSkuNames(loadedRules, lookupSkuNameFast);
    seedMonitorRuleSkuNameCache(loadedRules, skuNameCache);
    rules.value = loadedRules;
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

  const payload = buildMonitorConfigPayload(cleanedWebhook, rules.value);

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
    OnAppEvent('monitor_alert_result', payload => {
      upsertRuleHit(normalizeMonitorHit(payload));
    })
  );
  void loadConfig();
});

onActivated(() => {
  if (!hasActivatedOnce) {
    hasActivatedOnce = true;
    return;
  }
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
    <NCard title="钉钉 Webhook" size="small">
      <NSpace vertical size="small">
        <NInput
          v-model:value="webhook"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 2 }"
          placeholder="填写钉钉机器人 webhook（有规则时必填）"
        />
      </NSpace>
    </NCard>

    <NCard title="监控规则" size="small">
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
        <MonitorRuleCard
          v-for="(rule, idx) in rules"
          :key="rule.key"
          :rule="rule"
          :index="idx"
          :hits="getRuleHits(rule)"
          :hit-limit-per-rule="hitLimitPerRule"
          @remove="removeRule(idx)"
          @queue-lookup="queueLookupSkuName(rule)"
          @lookup-now="lookupSkuName(rule)"
          @copy-link="copyHitLink"
        />
      </NSpace>
    </NCard>
  </NSpace>
</template>
