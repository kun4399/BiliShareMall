<script setup lang="ts">
import { useScrapyTasks } from '@/features/scrapy/useScrapyTasks';
import { Play, StopSharp } from '@vicons/ionicons5';

const {
  finishTimeHash,
  failedTimeHash,
  retryStateHash,
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
  getOptionLabel,
  displayLabel,
  getCompletedRoundCount,
  addScrapy,
  handleClose,
  handleRun,
  handldStop
} = useScrapyTasks();
</script>

<template>
  <NSpace vertical size="large">
    <NAlert v-if="sourceNotice" title="筛选配置提醒" type="warning">
      {{ sourceNotice }}
    </NAlert>

    <NCard class="card-wrapper" title="添加爬取任务">
      <template #header-extra>
        <NButton @click="addScrapy">
          <template #icon>
            <icon-ic-round-plus />
          </template>
          添加
        </NButton>
      </template>

      <NSpace vertical size="large">
        <NCollapse default-expanded-names="category">
          <NCollapseItem title="类型" name="category">
            <NSelect
              v-model:value="selectedProduct"
              :options="productOptions"
              label-field="label"
              value-field="value"
              placeholder="选择类型"
            />
            <template #header-extra>{{ getOptionLabel(productOptions, selectedProduct) }}</template>
          </NCollapseItem>

          <NCollapseItem title="顺序" name="order">
            <NSelect
              v-model:value="selectedOrder"
              :options="orderOptions"
              label-field="label"
              value-field="value"
              placeholder="选择顺序"
            />
            <template #header-extra>{{ getOptionLabel(orderOptions, selectedOrder) }}</template>
          </NCollapseItem>

          <NCollapseItem title="价格筛选" name="price">
            <NSelect
              v-model:value="selectedPriceFilter"
              :options="priceFilterOptions"
              label-field="label"
              value-field="value"
              placeholder="选择价格筛选"
            />
            <template #header-extra>{{ getOptionLabel(priceFilterOptions, selectedPriceFilter) }}</template>
          </NCollapseItem>

          <NCollapseItem title="折扣筛选" name="discount">
            <NSelect
              v-model:value="selectedDiscountFilter"
              :options="discountFilterOptions"
              label-field="label"
              value-field="value"
              placeholder="选择折扣筛选"
            />
            <template #header-extra>{{ getOptionLabel(discountFilterOptions, selectedDiscountFilter) }}</template>
          </NCollapseItem>
        </NCollapse>
      </NSpace>
    </NCard>

    <NCard class="running-card" title="运行中的任务">
      <NSpace align="center">
        <NTag type="success" size="large">运行中 {{ runningCount }} 个</NTag>
        <NTag v-for="id in runningTaskIds" :key="id" type="info" round>
          任务 #{{ id }}
        </NTag>
      </NSpace>
    </NCard>

    <NCard
      v-for="(scrapy, idx) in scrapyList"
      :key="scrapy.id"
      :title="`${scrapy.productName} ${getOptionLabel(orderOptions, scrapy.order)}`"
      closable
      @close="() => handleClose(idx)"
    >
      <NSpace vertical size="large">
        <NAlert v-if="isTaskRunning(scrapy.id)" title="任务状态" type="success">
          正在运行中
        </NAlert>
        <NAlert v-if="retryStateHash[scrapy.id]" title="重试中" type="warning">
          {{ retryStateHash[scrapy.id]?.seconds }} 秒后重试，原因：{{ retryStateHash[scrapy.id]?.reason }}
        </NAlert>
        <NAlert v-if="finishTimeHash[scrapy.id]" title="本轮完成" type="success">
          时间：{{ finishTimeHash[scrapy.id] }}
        </NAlert>
        <NAlert v-if="failedTimeHash[scrapy.id]" title="执行失败" type="error">
          错误时间：{{ failedTimeHash[scrapy.id] }}
        </NAlert>
        <NSpace justify="space-around" size="large">
          <NStatistic label="折扣" :value="displayLabel(scrapy.discountFilterLabel)" />
          <NStatistic label="价格" :value="displayLabel(scrapy.priceFilterLabel)" />
          <NStatistic label="爬取次数" :value="scrapy.nums" />
          <NStatistic label="完成循环次数" :value="getCompletedRoundCount(scrapy)" />
          <NButton
            v-if="!isTaskRunning(scrapy.id)"
            class="custom-button"
            strong
            ghost
            circle
            round
            size="large"
            @click="() => handleRun(idx)"
          >
            <template #icon>
              <NIcon><Play /></NIcon>
            </template>
          </NButton>
          <NButton
            v-else
            class="custom-button"
            strong
            ghost
            circle
            round
            size="large"
            @click="() => handldStop(scrapy.id)"
          >
            <template #icon>
              <NIcon><StopSharp /></NIcon>
            </template>
          </NButton>
        </NSpace>
      </NSpace>

      <template #header-extra>
        <NFlex>
          <NTime class="custom-time" :time="new Date(scrapy.createTime)" />
        </NFlex>
      </template>
    </NCard>
  </NSpace>
</template>

<style lang="css">
.custom-button {
  margin-top: 12px;
}

.custom-time {
  color: gray;
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
