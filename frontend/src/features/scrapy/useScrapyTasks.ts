import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import type { scrapy } from '~/wailsjs/go/models';
import { dao } from '~/wailsjs/go/models';
import {
  CreateScrapyItem,
  DeleteScrapyItem,
  DoneTask,
  GetMarketRuntimeConfig,
  GetRunningTaskIds,
  OnAppEvent,
  ReadAllScrapyItems,
  StartTask
} from '@/gateway';
import { getToken } from '@/store/modules/auth/shared';

interface TimeHash {
  [key: number]: Date | undefined;
}

interface RetryState {
  seconds: number;
  reason: string;
  updatedAt: Date;
}

interface RetryHash {
  [key: number]: RetryState | undefined;
}

interface ScrapyRetryEvent {
  taskId: number;
  seconds: number;
  reason: string;
}

interface ScrapyRoundEvent {
  taskId: number;
  completedAt: number;
}

export function useScrapyTasks() {
  const message = useMessage();
  const loadingBar = useLoadingBar();

  const finishTimeHash = ref<TimeHash>({});
  const failedTimeHash = ref<TimeHash>({});
  const retryStateHash = ref<RetryHash>({});
  const scrapyList = ref<dao.ScrapyItem[]>([]);
  const runningTaskIds = ref<number[]>([]);
  const runtimeConfig = ref<scrapy.MarketRuntimeConfig | null>(null);
  const selectedProduct = ref('');
  const selectedOrder = ref('TIME_DESC');
  const selectedPriceFilter = ref('');
  const selectedDiscountFilter = ref('');

  const productOptions = computed(() => runtimeConfig.value?.categories ?? []);
  const orderOptions = computed(() => runtimeConfig.value?.sorts ?? []);
  const priceFilterOptions = computed(() => runtimeConfig.value?.priceFilters ?? []);
  const discountFilterOptions = computed(() => runtimeConfig.value?.discountFilters ?? []);
  const runningCount = computed(() => runningTaskIds.value.length);

  const sourceNotice = computed(() => {
    if (!runtimeConfig.value || runtimeConfig.value.source !== 'fallback') return '';
    return runtimeConfig.value.message || '当前使用内置筛选配置';
  });

  function getOptionLabel(options: scrapy.MarketFilterOption[], value: string) {
    if (value === '') return '不限';
    return options.find(option => option.value === value)?.label || value || '不限';
  }

  function displayLabel(label: string) {
    return label || '不限';
  }

  function getCompletedRoundCount(item: dao.ScrapyItem) {
    return item.increaseNumber || 0;
  }

  function isTaskRunning(taskID: number) {
    return runningTaskIds.value.includes(taskID);
  }

  function addRunningTask(taskID: number) {
    if (isTaskRunning(taskID)) return;
    runningTaskIds.value = [...runningTaskIds.value, taskID].sort((a, b) => a - b);
  }

  function removeRunningTask(taskID: number) {
    runningTaskIds.value = runningTaskIds.value.filter(id => id !== taskID);
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

  async function getAllItems() {
    const result = await ReadAllScrapyItems();
    return result.slice();
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
    const taskID = scrapyList.value[idx].id;
    if (isTaskRunning(taskID)) {
      message.warning('请先停止该任务');
      return;
    }
    loadingBar.start();
    DeleteScrapyItem(taskID)
      .then(() => {
        getAllItems().then(value => {
          scrapyList.value = value.slice();
          loadingBar.finish();
          message.success('删除成功');
        });
      })
      .catch(err => {
        loadingBar.error();
        message.error(err?.message || '删除失败');
      });
  }

  function handleRun(idx: number) {
    const taskID = scrapyList.value[idx].id;
    if (isTaskRunning(taskID)) {
      message.warning('该任务已在运行');
      return;
    }
    loadingBar.start();
    StartTask(taskID, getToken())
      .then(() => {
        addRunningTask(taskID);
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
        removeRunningTask(id);
        loadingBar.finish();
        message.success('已停止');
      })
      .catch(err => {
        loadingBar.error();
        message.error(err?.message || '停止失败');
      });
  }

  function parseTaskID(payload: unknown): number {
    if (typeof payload === 'number') return payload;
    if (payload && typeof payload === 'object' && 'taskId' in payload) {
      return Number((payload as { taskId?: number }).taskId || 0);
    }
    return 0;
  }

  const unlisteners: Array<() => void> = [];

  function setupEvents() {
    unlisteners.push(
      OnAppEvent('updateScrapyItem', payload => {
        const item = dao.ScrapyItem.createFrom(payload);
        const idx = scrapyList.value.findIndex(it => it.id === item.id);
        if (idx >= 0) {
          scrapyList.value[idx] = item;
        }
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_failed', payload => {
        message.error('任务失败，请稍后重试');
        const id = parseTaskID(payload);
        if (!id) return;
        failedTimeHash.value[id] = new Date();
        removeRunningTask(id);
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_round_finished', payload => {
        const event = payload as ScrapyRoundEvent;
        const id = parseTaskID(event);
        if (!id) return;
        finishTimeHash.value[id] = new Date(event.completedAt || Date.now());
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_finished', payload => {
        const id = parseTaskID(payload);
        if (!id) return;
        finishTimeHash.value[id] = new Date();
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_retry_wait', payload => {
        const event = payload as ScrapyRetryEvent;
        const id = parseTaskID(event);
        if (!id) return;
        retryStateHash.value[id] = {
          seconds: Number(event.seconds || 10),
          reason: event.reason || '请求失败',
          updatedAt: new Date()
        };
      })
    );

    unlisteners.push(
      OnAppEvent('scrapyItem_get_failed', payload => {
        const id = parseTaskID(payload);
        if (id) {
          removeRunningTask(id);
        }
        message.warning('当前爬取配置读取失败');
      })
    );
  }

  onMounted(async () => {
    setupEvents();
    loadingBar.start();
    await loadRuntimeConfig();
    scrapyList.value = await getAllItems();
    runningTaskIds.value = await GetRunningTaskIds();
    loadingBar.finish();
  });

  onUnmounted(() => {
    while (unlisteners.length > 0) {
      const unlisten = unlisteners.pop();
      if (unlisten) {
        unlisten();
      }
    }
  });

  return {
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
  };
}
