<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import { scrapy } from '~/wailsjs/go/models';
import { GetC2CItemNameBySku, GetMonitorConfig, SaveMonitorConfig } from '~/wailsjs/go/app/App';

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
}

const message = useMessage();
const loadingBar = useLoadingBar();

const webhook = ref('');
const rules = ref<MonitorRuleForm[]>([]);
const skuLookupTimerMap = new Map<number, ReturnType<typeof setTimeout>>();
let ruleKeySeed = 1;

const enabledRuleCount = computed(() => rules.value.filter(rule => rule.enabled).length);

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
    skuLookupSeq: 0
  };
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
    return;
  }

  rule.skuNameLoading = true;
  rule.skuLookupSeq += 1;
  const currentSeq = rule.skuLookupSeq;

  try {
    const name = (await GetC2CItemNameBySku(skuId)).trim();
    if (rule.skuLookupSeq !== currentSeq) {
      return;
    }
    rule.skuName = name;
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
  }, 280);
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

function removeRule(index: number) {
  const rule = rules.value[index];
  if (rule) {
    clearSkuLookupTimer(rule.key);
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
  loadConfig();
});

onUnmounted(() => {
  skuLookupTimerMap.forEach(timer => {
    clearTimeout(timer);
  });
  skuLookupTimerMap.clear();
});
</script>

<template>
  <NSpace vertical size="large">
    <NCard title="钉钉监控告警">
      <NSpace vertical size="large">
        <NInput
          v-model:value="webhook"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 4 }"
          placeholder="填写钉钉机器人 webhook（有规则时必填）"
        />
        <NAlert type="info" title="规则说明">
          当前共 {{ rules.length }} 条规则，启用 {{ enabledRuleCount }} 条。价格输入单位为“元”。
        </NAlert>
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
      <NSpace v-else vertical size="large">
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
          <NGrid :cols="3" :x-gap="12" :y-gap="12">
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
            <NFormItemGi label="商品名" :span="3">
              <NText :depth="rule.skuName ? 2 : 3">{{ getSkuNameDisplayText(rule) }}</NText>
            </NFormItemGi>
          </NGrid>
        </NCard>
      </NSpace>
    </NCard>
  </NSpace>
</template>
