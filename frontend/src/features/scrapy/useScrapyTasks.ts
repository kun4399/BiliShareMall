import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useLoadingBar, useMessage } from 'naive-ui';
import type { auth, scrapy } from '~/wailsjs/go/models';
import { dao } from '~/wailsjs/go/models';
import {
  CreateScrapyItem,
  DeleteScrapyItem,
  DoneTask,
  GetMarketRuntimeConfig,
  GetRunningTaskIds,
  ListLoginAccounts,
  OnAppEvent,
  ReadAllScrapyItems,
  StartTask,
  UpdateScrapyTaskConfig
} from '@/gateway';
import { getToken } from '@/store/modules/auth/shared';
import {
  applyTaskUiStateTransition,
  createTaskUiState,
  createTaskUiStateMap,
  type TaskUiEvent,
  type TaskUiState
} from './task-state';

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

  const taskStateMap = ref<Record<number, TaskUiState>>({});
  const scrapyList = ref<dao.ScrapyItem[]>([]);
  const loginAccounts = ref<auth.LoginAccount[]>([]);
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
  const accountOptions = computed(() => {
    return [
      { label: '不绑定账号（默认会话）', value: 0 },
      ...loginAccounts.value.map(account => ({
        label: `${account.accountName || account.uid} (${account.loggedIn ? '已登录' : '未登录'})`,
        value: account.id
      }))
    ];
  });

  const sourceNotice = computed(() => {
    if (!runtimeConfig.value || runtimeConfig.value.source !== 'fallback') return '';
    return runtimeConfig.value.message || '当前使用内置筛选配置';
  });

  function getOptionLabel(options: scrapy.MarketFilterOption[], value: string) {
    if (value === '') return '不限';
    return options.find(option => option.value === value)?.label || value || '不限';
  }

  function getAccountLabelById(accountID: number, fallback = '') {
    if (!accountID) {
      return fallback || '未绑定账号';
    }
    const account = loginAccounts.value.find(item => item.id === accountID);
    if (!account) {
      return fallback || `账号#${accountID}`;
    }
    return account.accountName || account.uid || fallback || `账号#${accountID}`;
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

  function updateTaskState(taskID: number, event: TaskUiEvent) {
    taskStateMap.value = {
      ...taskStateMap.value,
      [taskID]: applyTaskUiStateTransition(taskStateMap.value[taskID], event)
    };
  }

  function clearTaskState(taskID: number) {
    if (!(taskID in taskStateMap.value)) {
      return;
    }
    const next = { ...taskStateMap.value };
    delete next[taskID];
    taskStateMap.value = next;
  }

  function getTaskUiState(taskID: number) {
    return taskStateMap.value[taskID] || createTaskUiState();
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

  async function loadLoginAccounts() {
    loginAccounts.value = await ListLoginAccounts();
  }

  function createScrapyPayload() {
    return dao.ScrapyItem.createFrom({
      product: selectedProduct.value,
      productName: getOptionLabel(productOptions.value, selectedProduct.value),
      accountId: 0,
      requestIntervalSeconds: 3,
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

  async function addScrapy() {
    const item = createScrapyPayload();
    try {
      const id = await CreateScrapyItem(item);
      if (id === -1) {
        message.error('添加失败');
        return;
      }
      item.id = id;
      scrapyList.value = await getAllItems();
      message.success('添加成功');
    } catch (err: any) {
      message.error(err?.message || '添加失败');
    }
  }

  async function handleClose(idx: number) {
    const taskID = scrapyList.value[idx].id;
    if (isTaskRunning(taskID)) {
      message.warning('请先停止该任务');
      return;
    }
    loadingBar.start();
    try {
      await DeleteScrapyItem(taskID);
      scrapyList.value = await getAllItems();
      clearTaskState(taskID);
      loadingBar.finish();
      message.success('删除成功');
    } catch (err: any) {
      loadingBar.error();
      message.error(err?.message || '删除失败');
    }
  }

  async function handleRun(idx: number) {
    const taskID = scrapyList.value[idx].id;
    if (isTaskRunning(taskID)) {
      message.warning('该任务已在运行');
      return;
    }
    loadingBar.start();
    try {
      await StartTask(taskID, getToken());
      addRunningTask(taskID);
      updateTaskState(taskID, { type: 'start' });
      loadingBar.finish();
      message.success('启动成功');
    } catch (err: any) {
      loadingBar.error();
      message.error(err?.message || '启动失败');
    }
  }

  async function handleSaveTaskConfig(taskID: number, accountID: number, requestIntervalSeconds: number) {
    const normalizedInterval = Number(requestIntervalSeconds.toFixed(1));
    if (normalizedInterval < 0) {
      message.warning('间隔不能小于 0');
      return;
    }
    try {
      await UpdateScrapyTaskConfig(taskID, accountID, normalizedInterval);
      scrapyList.value = await getAllItems();
      message.success('配置已更新');
    } catch (err: any) {
      message.error(err?.message || '配置更新失败');
    }
  }

  async function handleStop(taskID: number) {
    loadingBar.start();
    try {
      await DoneTask(taskID);
      removeRunningTask(taskID);
      updateTaskState(taskID, { type: 'stop' });
      loadingBar.finish();
      message.success('已停止');
    } catch (err: any) {
      loadingBar.error();
      message.error(err?.message || '停止失败');
    }
  }

  function parseTaskID(payload: unknown): number {
    if (typeof payload === 'number') return payload;
    if (payload && typeof payload === 'object' && 'taskId' in payload) {
      return Number((payload as { taskId?: number }).taskId || 0);
    }
    return 0;
  }

  const unlisteners: Array<() => void> = [];
  const onAccountsUpdated = () => {
    void loadLoginAccounts();
  };

  function setupEvents() {
    unlisteners.push(
      OnAppEvent('updateScrapyItem', payload => {
        const item = dao.ScrapyItem.createFrom(payload);
        const idx = scrapyList.value.findIndex(it => it.id === item.id);
        if (idx >= 0) {
          scrapyList.value = scrapyList.value.map(it => (it.id === item.id ? item : it));
        }
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_failed', payload => {
        message.error('任务失败，请稍后重试');
        const id = parseTaskID(payload);
        if (!id) return;
        removeRunningTask(id);
        updateTaskState(id, { type: 'failed' });
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_round_finished', payload => {
        const event = payload as ScrapyRoundEvent;
        const id = parseTaskID(event);
        if (!id) return;
        updateTaskState(id, {
          type: 'completed',
          at: Number(event.completedAt || Date.now())
        });
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_finished', payload => {
        const id = parseTaskID(payload);
        if (!id) return;
        updateTaskState(id, { type: 'completed' });
      })
    );

    unlisteners.push(
      OnAppEvent('scrapy_retry_wait', payload => {
        const event = payload as ScrapyRetryEvent;
        const id = parseTaskID(event);
        if (!id) return;
        addRunningTask(id);
        updateTaskState(id, {
          type: 'retry_wait',
          seconds: Number(event.seconds || 10),
          reason: event.reason || '请求失败'
        });
      })
    );

    unlisteners.push(
      OnAppEvent('scrapyItem_get_failed', payload => {
        const id = parseTaskID(payload);
        if (id) {
          removeRunningTask(id);
          updateTaskState(id, { type: 'failed' });
        }
        message.warning('当前爬取配置读取失败');
      })
    );
  }

  onMounted(async () => {
    setupEvents();
    window.addEventListener('bsm-login-accounts-updated', onAccountsUpdated);
    loadingBar.start();
    await loadLoginAccounts();
    await loadRuntimeConfig();
    scrapyList.value = await getAllItems();
    runningTaskIds.value = await GetRunningTaskIds();
    taskStateMap.value = createTaskUiStateMap(runningTaskIds.value);
    loadingBar.finish();
  });

  onUnmounted(() => {
    window.removeEventListener('bsm-login-accounts-updated', onAccountsUpdated);
    while (unlisteners.length > 0) {
      const unlisten = unlisteners.pop();
      if (unlisten) {
        unlisten();
      }
    }
  });

  return {
    taskStateMap,
    scrapyList,
    runningTaskIds,
    runningCount,
    loginAccounts,
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
    getTaskUiState,
    getOptionLabel,
    getAccountLabelById,
    displayLabel,
    getCompletedRoundCount,
    addScrapy,
    handleSaveTaskConfig,
    handleClose,
    handleRun,
    handleStop
  };
}
