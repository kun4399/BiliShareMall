<script setup lang="ts">
import { useScrapyTasks } from '@/features/scrapy/useScrapyTasks';
import ScrapyTaskCard from './modules/scrapy-task-card.vue';

const {
  scrapyList,
  runningTaskIds,
  runningCount,
  accountOptions,
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
  isTaskActionPending,
  getTaskUiState,
  getOptionLabel,
  getAccountLabelById,
  addScrapy,
  handleSaveTaskConfig,
  handleClose,
  handleRun,
  handleStop
} = useScrapyTasks();
</script>

<template>
  <main class="scrapy-page">
    <section class="bsm-page-heading">
      <div>
        <p class="bsm-page-heading__eyebrow">COLLECTION TASKS</p>
        <h1>爬取任务</h1>
        <p>配置账号与筛选条件，持续采集会员购市集商品。</p>
      </div>
      <NTag :type="runningCount ? 'success' : 'default'" round size="large">
        {{ runningCount ? `${runningCount} 个任务运行中` : '当前无运行任务' }}
      </NTag>
    </section>

    <NAlert v-if="sourceNotice" title="筛选配置提醒" type="warning">
      {{ sourceNotice }}
    </NAlert>

    <NCard class="bsm-surface-card card-wrapper" title="添加爬取任务" size="small" :bordered="false">
      <template #header-extra>
        <NButton @click="addScrapy">
          <template #icon>
            <icon-ic-round-plus />
          </template>
          添加
        </NButton>
      </template>

      <NGrid cols="1 s:2 l:4" responsive="screen" :x-gap="10" :y-gap="6">
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

    <NCard v-if="runningCount" class="running-card" title="运行中的任务" size="small" :bordered="false">
      <NSpace align="center" size="small">
        <NTag type="success" size="medium">运行中 {{ runningCount }} 个</NTag>
        <NTag v-for="id in runningTaskIds" :key="id" type="info" round>任务 #{{ id }}</NTag>
      </NSpace>
    </NCard>

    <div v-if="scrapyList.length" class="scrapy-task-list">
      <ScrapyTaskCard
        v-for="(scrapy, idx) in scrapyList"
        :key="scrapy.id"
        :task="scrapy"
        :order-label="getOptionLabel(orderOptions, scrapy.order)"
        :account-label="getAccountLabelById(scrapy.accountId, scrapy.accountName)"
        :account-options="accountOptions"
        :task-state="getTaskUiState(scrapy.id)"
        :is-running="isTaskRunning(scrapy.id)"
        :action-pending="isTaskActionPending(scrapy.id)"
        @close="handleClose(idx)"
        @run="handleRun(idx)"
        @stop="handleStop(scrapy.id)"
        @save-config="handleSaveTaskConfig(scrapy.id, $event.accountId, $event.requestIntervalSeconds)"
      />
    </div>

    <NEmpty v-else class="scrapy-empty" description="还没有爬取任务，请先添加一个任务" />
  </main>
</template>

<style lang="css">
.scrapy-page {
  display: flex;
  flex-direction: column;
  gap: var(--bsm-space-4);
  width: min(100%, 1500px);
  margin: 0 auto;
}

.scrapy-task-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 480px), 1fr));
  gap: 14px;
}

.scrapy-empty {
  padding: 64px 16px;
  border: 1px dashed var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  background: var(--bsm-surface);
}

.card-wrapper :is(.n-card__content, .n-card-header) {
  padding-bottom: 8px;
}

.task-form-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 2px;
}

.running-card {
  --n-color: #dbf5ca;
  --n-color-modal: #dbf5ca;
  --n-border-color: #c3e6b8;
  --n-text-color: #1f3d18;
  --n-title-text-color: #1f3d18;
  color: #1f3d18;
  border-radius: var(--bsm-radius-lg);
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

@media (max-width: 640px) {
  .scrapy-task-list {
    grid-template-columns: 1fr;
  }
}
</style>
