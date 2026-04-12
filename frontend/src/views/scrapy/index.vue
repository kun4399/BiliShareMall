<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import { Play, StopSharp } from '@vicons/ionicons5';
import {
  CreateScrapyItem,
  DeleteScrapyItem,
  DoneTask,
  GetMarketRuntimeConfig,
  GetNowRunTaskId,
  ReadAllScrapyItems,
  StartTask
} from '~/wailsjs/go/app/App';
import { app, dao } from '~/wailsjs/go/models';
import { getToken } from '@/store/modules/auth/shared';
import { EventsOn } from '~/wailsjs/runtime/runtime';

const message = useMessage();
const loadingBar = useLoadingBar();

interface TimeHash {
  [key: number]: Date | undefined;
}

const finishTimeHash = ref<TimeHash>({});
const failedTimeHash = ref<TimeHash>({});
const nowIdx = ref(-1);
const scrapyList = ref<dao.ScrapyItem[]>([]);
const runtimeConfig = ref<app.MarketRuntimeConfig | null>(null);
const selectedProduct = ref('');
const selectedOrder = ref('TIME_DESC');
const selectedPriceFilter = ref('');
const selectedDiscountFilter = ref('');

const productOptions = computed(() => runtimeConfig.value?.categories ?? []);
const orderOptions = computed(() => runtimeConfig.value?.sorts ?? []);
const priceFilterOptions = computed(() => runtimeConfig.value?.priceFilters ?? []);
const discountFilterOptions = computed(() => runtimeConfig.value?.discountFilters ?? []);

const sourceNotice = computed(() => {
  if (!runtimeConfig.value || runtimeConfig.value.source !== 'fallback') return '';
  return runtimeConfig.value.message || '当前使用内置筛选配置';
});

function getOptionLabel(options: app.MarketFilterOption[], value: string) {
  if (value === '') return '不限';
  return options.find(option => option.value === value)?.label || value || '不限';
}

function displayLabel(label: string) {
  return label || '不限';
}

async function loadRuntimeConfig() {
  const config = await GetMarketRuntimeConfig(getToken());
  runtimeConfig.value = config;

  if (!selectedProduct.value && config.categories.length > 0) {
    selectedProduct.value = config.categories[0].value;
  }
  if (!selectedOrder.value && config.sorts.length > 0) {
    selectedOrder.value = config.sorts[0].value;
  }
}

function createScrapyPayload() {
  return dao.ScrapyItem.createFrom({
    product: selectedProduct.value,
    productName: getOptionLabel(productOptions.value, selectedProduct.value),
    order: selectedOrder.value,
    priceFilter: selectedPriceFilter.value,
    priceFilterLabel: getOptionLabel(priceFilterOptions.value, selectedPriceFilter.value),
    discountFilter: selectedDiscountFilter.value,
    discountFilterLabel: getOptionLabel(discountFilterOptions.value, selectedDiscountFilter.value),
    nums: 0,
    increaseNumber: 0,
    nextToken: ''
  });
}

function addScrapy() {
  const item = createScrapyPayload();
  CreateScrapyItem(item).then(id => {
    if (id === -1) {
      message.error('添加失败');
      return;
    }
    item.id = id;
    getAllItems().then(value => {
      scrapyList.value = value.slice();
      message.success('添加成功');
    });
  });
}

function handleClose(idx: number) {
  if (nowIdx.value !== -1) {
    message.warning('请先关闭当前爬虫任务');
    return;
  }
  loadingBar.start();
  DeleteScrapyItem(scrapyList.value[idx].id)
    .then(() => {
      getAllItems().then(value => {
        scrapyList.value = value.slice();
        loadingBar.finish();
        message.success('删除成功');
      });
    })
    .catch(() => {
      loadingBar.error();
      message.error('删除失败');
    });
}

function handleRun(idx: number) {
  if (nowIdx.value === idx) {
    message.warning('该任务已在运行');
    return;
  }
  loadingBar.start();
  StartTask(scrapyList.value[idx].id, getToken())
    .then(() => {
      nowIdx.value = idx;
      loadingBar.finish();
      message.success('启动成功');
    })
    .catch(err => {
      loadingBar.error();
      message.error(err?.message || '启动失败');
    });
}

function handldStop(id: number) {
  loadingBar.start();
  DoneTask(id)
    .then(() => {
      nowIdx.value = -1;
      loadingBar.finish();
      message.success('已停止');
    })
    .catch(() => {
      loadingBar.error();
      message.error('停止失败');
    });
}

async function getAllItems() {
  const result = await ReadAllScrapyItems();
  return result.slice();
}

EventsOn('updateScrapyItem', payload => {
  const item = dao.ScrapyItem.createFrom(payload);
  const idx = scrapyList.value.findIndex(it => it.id === item.id);
  scrapyList.value[idx] = item;
  nowIdx.value = idx;
});

EventsOn('scrapy_failed', payload => {
  message.error('任务失败，请检查登录状态或稍后重试');
  const id = payload as number;
  failedTimeHash.value[id] = new Date();
  nowIdx.value = -1;
});

EventsOn('scrapy_finished', payload => {
  const id = payload as number;
  finishTimeHash.value[id] = new Date();
  nowIdx.value = -1;
});

EventsOn('scrapy_wait', payload => {
  const second = payload as number;
  message.warning(`出现风控，等待 ${second} 秒后继续`);
});

EventsOn('scrapyItem_get_failed', _ => {
  message.warning('当前爬取配置读取失败');
});

onMounted(async () => {
  loadingBar.start();
  await loadRuntimeConfig();
  scrapyList.value = await getAllItems();
  const nowRunTaskId = await GetNowRunTaskId();
  scrapyList.value.forEach((item, index) => {
    if (item.id === nowRunTaskId) {
      nowIdx.value = index;
    }
  });
  loadingBar.finish();
});
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
