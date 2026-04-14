<script setup lang="ts">
import { useScrapyTasks } from '@/features/scrapy/useScrapyTasks';
import ScrapyTaskCard from './modules/scrapy-task-card.vue';

const {
  scrapyList,
  runningTaskIds,
  runningCount,
  selectedProduct,
  selectedOrder,
  selectedPriceFilter,
  selectedDiscountFilter,
  productOptions,
  orderOptions,
  priceFilterOptions,
  discountFilterOptions,
  sourceNotice,
  isTaskRunning,
  getTaskUiState,
  getOptionLabel,
  addScrapy,
  handleClose,
  handleRun,
  handleStop
} = useScrapyTasks();
</script>

<template>
  <NSpace vertical size="medium">
    <NAlert v-if="sourceNotice" title="筛选配置提醒" type="warning">
      {{ sourceNotice }}
    </NAlert>

    <NCard class="card-wrapper" title="添加爬取任务" size="small">
      <template #header-extra>
        <NButton @click="addScrapy">
          <template #icon>
            <icon-ic-round-plus />
          </template>
          添加
        </NButton>
      </template>

      <NGrid cols="1 s:2 l:4" responsive="screen" :x-gap="12" :y-gap="8">
        <NFormItemGi label="类型">
          <NSelect
            v-model:value="selectedProduct"
            :options="productOptions"
            label-field="label"
            value-field="value"
            placeholder="选择类型"
          />
        </NFormItemGi>

        <NFormItemGi label="顺序">
          <NSelect
            v-model:value="selectedOrder"
            :options="orderOptions"
            label-field="label"
            value-field="value"
            placeholder="选择顺序"
          />
        </NFormItemGi>

        <NFormItemGi label="价格筛选">
          <NSelect
            v-model:value="selectedPriceFilter"
            :options="priceFilterOptions"
            label-field="label"
            value-field="value"
            placeholder="选择价格筛选"
          />
        </NFormItemGi>

        <NFormItemGi label="折扣筛选">
          <NSelect
            v-model:value="selectedDiscountFilter"
            :options="discountFilterOptions"
            label-field="label"
            value-field="value"
            placeholder="选择折扣筛选"
          />
        </NFormItemGi>
      </NGrid>

      <div class="task-form-summary">
        <NTag size="small" round>{{ getOptionLabel(productOptions, selectedProduct) }}</NTag>
        <NTag size="small" round>{{ getOptionLabel(orderOptions, selectedOrder) }}</NTag>
        <NTag size="small" round>{{ getOptionLabel(priceFilterOptions, selectedPriceFilter) }}</NTag>
        <NTag size="small" round>{{ getOptionLabel(discountFilterOptions, selectedDiscountFilter) }}</NTag>
      </div>
    </NCard>

    <NCard class="running-card" title="运行中的任务" size="small">
      <NSpace align="center" size="small">
        <NTag type="success" size="medium">运行中 {{ runningCount }} 个</NTag>
        <NTag v-for="id in runningTaskIds" :key="id" type="info" round>
          任务 #{{ id }}
        </NTag>
      </NSpace>
    </NCard>

    <ScrapyTaskCard
      v-for="(scrapy, idx) in scrapyList"
      :key="scrapy.id"
      :task="scrapy"
      :order-label="getOptionLabel(orderOptions, scrapy.order)"
      :task-state="getTaskUiState(scrapy.id)"
      :is-running="isTaskRunning(scrapy.id)"
      @close="handleClose(idx)"
      @run="handleRun(idx)"
      @stop="handleStop(scrapy.id)"
    />
  </NSpace>
</template>

<style lang="css">
.card-wrapper :is(.n-card__content, .n-card-header) {
  padding-bottom: 12px;
}

.task-form-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 4px;
}

.running-card {
  --n-color: #dbf5ca;
  --n-color-modal: #dbf5ca;
  --n-border-color: #c3e6b8;
  --n-text-color: #1f3d18;
  --n-title-text-color: #1f3d18;
  color: #1f3d18;
}

.running-card :is(.n-card-header__main, .n-card-header__extra, .n-card__content) {
  color: inherit;
}

html.dark .running-card {
  --n-color: #24412f;
  --n-color-modal: #24412f;
  --n-border-color: #3a6a48;
  --n-text-color: #e8f7ea;
  --n-title-text-color: #e8f7ea;
  color: #e8f7ea;
}
</style>
