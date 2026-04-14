<script setup lang="ts">
import { computed } from 'vue';
import { NButton, NCard, NCollapse, NCollapseItem, NList, NListItem, NSwitch, NTag, NText } from 'naive-ui';
import type { scrapy } from '~/wailsjs/go/models';
import {
  formatMonitorHitPrice,
  formatMonitorHitTime,
  getSkuNameDisplayText,
  summarizeRuleHits,
  type MonitorRuleForm
} from '@/features/monitor/rule-editor';

defineOptions({
  name: 'MonitorRuleCard'
});

const props = defineProps<{
  rule: MonitorRuleForm;
  index: number;
  hits: scrapy.MonitorHitItem[];
  hitLimitPerRule: number;
}>();

const emit = defineEmits<{
  remove: [];
  queueLookup: [];
  lookupNow: [];
  copyLink: [link: string];
}>();

const hitSummary = computed(() => summarizeRuleHits(props.hits, props.hitLimitPerRule));

function hitKey(hit: scrapy.MonitorHitItem, index: number) {
  return `${hit.ruleId}-${hit.c2cItemsId}-${hit.status}-${hit.occurredAt}-${index}`;
}
</script>

<template>
  <NCard class="monitor-rule-card" size="small" :title="`规则 #${index + 1}`">
    <template #header-extra>
      <div class="monitor-rule-header-extra">
        <NText depth="3" class="monitor-rule-summary">{{ hitSummary.summaryText }}</NText>
        <NTag v-if="hitSummary.count > 0" type="info" size="small" round>{{ hitSummary.count }}</NTag>
        <NText depth="3">启用</NText>
        <NSwitch v-model:value="rule.enabled" />
        <NButton quaternary type="error" @click="emit('remove')">删除</NButton>
      </div>
    </template>

    <NGrid cols="1 s:2 l:4" responsive="screen" :x-gap="10" :y-gap="6">
      <NFormItemGi label="SKU ID">
        <NInputNumber
          v-model:value="rule.skuId"
          :min="1"
          :precision="0"
          placeholder="如 123456"
          @update:value="emit('queueLookup')"
          @blur="emit('lookupNow')"
        />
      </NFormItemGi>

      <NFormItemGi label="商品名">
        <NText :depth="rule.skuName ? 2 : 3" class="monitor-rule-name">
          {{ getSkuNameDisplayText(rule) }}
        </NText>
      </NFormItemGi>

      <NFormItemGi label="最低价（元）">
        <NInputNumber v-model:value="rule.minPriceYuan" :min="0" :precision="2" placeholder="如 99.00" />
      </NFormItemGi>

      <NFormItemGi label="最高价（元）">
        <NInputNumber v-model:value="rule.maxPriceYuan" :min="0" :precision="2" placeholder="如 199.00" />
      </NFormItemGi>
    </NGrid>

    <div class="monitor-hit-section">
      <NText depth="3">命中记录</NText>

      <NText v-if="rule.id <= 0" depth="3" class="monitor-hit-empty">
        保存规则后开始记录命中结果
      </NText>

      <NText v-else-if="hits.length === 0" depth="3" class="monitor-hit-empty">
        暂无命中记录
      </NText>

      <NCollapse v-else class="monitor-hit-collapse">
        <NCollapseItem name="hits" :title="hitSummary.summaryText">
          <NList bordered size="small">
            <NListItem v-for="(hit, hitIdx) in hits" :key="hitKey(hit, hitIdx)">
              <div class="monitor-hit-item">
                <div class="monitor-hit-main">
                  <NText depth="3">{{ formatMonitorHitPrice(hit) }} 元</NText>
                  <NText depth="3">{{ formatMonitorHitTime(hit.occurredAt) }}</NText>
                  <NTag :type="hit.status === 'failed' ? 'error' : 'success'" size="small" round>
                    {{ hit.status === 'failed' ? '发送失败' : '已发送' }}
                  </NTag>
                </div>
                <NButton text type="primary" @click="emit('copyLink', hit.itemLink)">链接</NButton>
              </div>
              <NText v-if="hit.errorMessage" depth="3" type="error">{{ hit.errorMessage }}</NText>
            </NListItem>
          </NList>
        </NCollapseItem>
      </NCollapse>
    </div>
  </NCard>
</template>

<style scoped>
.monitor-rule-card :deep(.n-card__content) {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.monitor-rule-header-extra {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.monitor-rule-summary {
  max-width: 320px;
  text-align: right;
}

.monitor-rule-name {
  line-height: 1.5;
}

.monitor-hit-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.monitor-hit-empty {
  font-size: 12px;
}

.monitor-hit-collapse :deep(.n-collapse-item__header) {
  padding-top: 6px;
  padding-bottom: 6px;
}

.monitor-hit-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.monitor-hit-main {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 640px) {
  .monitor-rule-header-extra {
    justify-content: flex-start;
  }

  .monitor-rule-summary {
    max-width: none;
    text-align: left;
    width: 100%;
  }

  .monitor-hit-item {
    align-items: flex-start;
  }
}
</style>
