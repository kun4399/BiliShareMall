<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import { scrapy } from '~/wailsjs/go/models';
import { GetMonitorConfig, SaveMonitorConfig } from '~/wailsjs/go/app/App';

interface MonitorRuleForm {
  id: number;
  skuId: number | null;
  minPriceYuan: number | null;
  maxPriceYuan: number | null;
  enabled: boolean;
}

const message = useMessage();
const loadingBar = useLoadingBar();

const webhook = ref('');
const rules = ref<MonitorRuleForm[]>([]);

const enabledRuleCount = computed(() => rules.value.filter(rule => rule.enabled).length);

function addRule() {
  rules.value.push({
    id: 0,
    skuId: null,
    minPriceYuan: null,
    maxPriceYuan: null,
    enabled: true
  });
}

function removeRule(index: number) {
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
    rules.value = (config.rules || []).map(rule => ({
      id: rule.id || 0,
      skuId: rule.skuId || null,
      minPriceYuan: toYuan(rule.minPrice || 0),
      maxPriceYuan: toYuan(rule.maxPrice || 0),
      enabled: rule.enabled ?? true
    }));
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
          :key="`${rule.id}-${idx}`"
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
              <NInputNumber v-model:value="rule.skuId" :min="1" :precision="0" placeholder="如 123456" />
            </NFormItemGi>
            <NFormItemGi label="最低价（元）">
              <NInputNumber v-model:value="rule.minPriceYuan" :min="0" :precision="2" placeholder="如 99.00" />
            </NFormItemGi>
            <NFormItemGi label="最高价（元）">
              <NInputNumber v-model:value="rule.maxPriceYuan" :min="0" :precision="2" placeholder="如 199.00" />
            </NFormItemGi>
          </NGrid>
        </NCard>
      </NSpace>
    </NCard>
  </NSpace>
</template>
