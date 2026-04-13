<script setup lang="ts">
import { useScrapyTasks } from '@/features/scrapy/useScrapyTasks';
import { Play, StopSharp } from '@vicons/ionicons5';
const {
  finishTimeHash,
  failedTimeHash,
  nowIdx,
  scrapyList,
  selectedProduct,
  selectedOrder,
  selectedPriceFilter,
  selectedDiscountFilter,
  productOptions,
  orderOptions,
  priceFilterOptions,
  discountFilterOptions,
  sourceNotice,
  getOptionLabel,
  displayLabel,
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

    <NCard class="running-card" title="当前运行">
      <NEmpty v-if="nowIdx === -1" description="暂无" />
      <div v-else>
        <NSpace justify="space-around" size="large">
          <NStatistic label="类型" :value="scrapyList[nowIdx].productName" />
          <NStatistic label="爬取顺序" :value="getOptionLabel(orderOptions, scrapyList[nowIdx].order)" />
          <NStatistic label="折扣" :value="displayLabel(scrapyList[nowIdx].discountFilterLabel)" />
          <NStatistic label="价格" :value="displayLabel(scrapyList[nowIdx].priceFilterLabel)" />
          <NStatistic label="爬取次数" :value="scrapyList[nowIdx].nums" />
          <NStatistic label="增加数目" :value="scrapyList[nowIdx].increaseNumber" />
          <NButton class="custom-button" strong ghost circle round size="large" @click="() => handldStop(scrapyList[nowIdx].id)">
            <template #icon>
              <NIcon><StopSharp /></NIcon>
            </template>
          </NButton>
        </NSpace>
      </div>
    </NCard>

    <NCard
      v-for="(scrapy, idx) in scrapyList"
      :key="scrapy.id"
      :title="`${scrapy.productName} ${getOptionLabel(orderOptions, scrapy.order)}`"
      closable
      @close="() => handleClose(idx)"
    >
      <NSpace vertical size="large">
        <NAlert v-if="finishTimeHash[scrapy.id]" title="执行完成" type="success">
          完成时间：{{ finishTimeHash[scrapy.id] }}
        </NAlert>
        <NAlert v-if="failedTimeHash[scrapy.id]" title="执行失败" type="error">
          错误时间：{{ failedTimeHash[scrapy.id] }}
        </NAlert>
        <NSpace justify="space-around" size="large">
          <NStatistic label="折扣" :value="displayLabel(scrapy.discountFilterLabel)" />
          <NStatistic label="价格" :value="displayLabel(scrapy.priceFilterLabel)" />
          <NStatistic label="爬取次数" :value="scrapy.nums" />
          <NStatistic label="增加数目" :value="scrapy.increaseNumber" />
          <NButton class="custom-button" strong ghost circle round size="large" @click="() => handleRun(idx)">
            <template #icon>
              <NIcon><Play /></NIcon>
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
  background-color: #dbf5ca;
  color: #333;
  border: 1px solid #ccc;
}
</style>
