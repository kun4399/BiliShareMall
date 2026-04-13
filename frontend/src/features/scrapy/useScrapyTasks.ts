import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import { dao, scrapy } from '~/wailsjs/go/models';
import { EventsOn } from '~/wailsjs/runtime/runtime';
import {
  CreateScrapyItem,
  DeleteScrapyItem,
  DoneTask,
  GetMarketRuntimeConfig,
  GetNowRunTaskId,
  ReadAllScrapyItems,
  StartTask
} from '~/wailsjs/go/app/App';
import { getToken } from '@/store/modules/auth/shared';

interface TimeHash {
  [key: number]: Date | undefined;
}

export function useScrapyTasks() {
  const message = useMessage();
  const loadingBar = useLoadingBar();

  const finishTimeHash = ref<TimeHash>({});
  const failedTimeHash = ref<TimeHash>({});
  const nowIdx = ref(-1);
  const scrapyList = ref<dao.ScrapyItem[]>([]);
  const runtimeConfig = ref<scrapy.MarketRuntimeConfig | null>(null);
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

  function getOptionLabel(options: scrapy.MarketFilterOption[], value: string) {
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

  const unlisteners: Array<() => void> = [];

  function setupEvents() {
    unlisteners.push(
      EventsOn('updateScrapyItem', payload => {
        const item = dao.ScrapyItem.createFrom(payload);
        const idx = scrapyList.value.findIndex(it => it.id === item.id);
        if (idx >= 0) {
          scrapyList.value[idx] = item;
          nowIdx.value = idx;
        }
      })
    );

    unlisteners.push(
      EventsOn('scrapy_failed', payload => {
        message.error('任务失败，请检查登录状态或稍后重试');
        const id = payload as number;
        failedTimeHash.value[id] = new Date();
        nowIdx.value = -1;
      })
    );

    unlisteners.push(
      EventsOn('scrapy_finished', payload => {
        const id = payload as number;
        finishTimeHash.value[id] = new Date();
        nowIdx.value = -1;
      })
    );

    unlisteners.push(
      EventsOn('scrapy_wait', payload => {
        const second = payload as number;
        message.warning(`出现风控，等待 ${second} 秒后继续`);
      })
    );

    unlisteners.push(
      EventsOn('scrapyItem_get_failed', _ => {
        message.warning('当前爬取配置读取失败');
      })
    );
  }

  onMounted(async () => {
    setupEvents();
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
  };
}
