import { computed, onActivated, onDeactivated, onMounted, onUnmounted, ref } from 'vue';
import { useMessage } from 'naive-ui';
import { useRouter } from 'vue-router';
import { EventsOn } from '~/wailsjs/runtime/runtime';
import type { catalog } from '~/wailsjs/go/models';
import { fetchCatalogList } from './api';

interface SortWay {
  value: number;
  label: string;
}

const REALTIME_REFRESH_DEBOUNCE_MS = 500;

export function useCatalogList() {
  const router = useRouter();
  const message = useMessage();

  const loading = ref(false);
  const searchText = ref('');
  const timeRange = ref<[number, number] | null>([1183135260000, Date.now()]);
  const timeRangeEnable = ref(false);
  const priceRangeEnable = ref(false);
  const priceRange = ref<[number | null, number | null]>([0, 9999]);
  const sortOpt = ref(1);
  const data = ref<catalog.C2CItemGroupVO[]>([]);
  const pagination = ref({
    page: 1,
    pageCount: 1,
    pageSize: 12,
    itemCount: 0
  });

  const sortways = ref<SortWay[]>([
    { value: 1, label: '最新上架' },
    { value: 2, label: '最低价升序' },
    { value: 3, label: '最低价降序' }
  ]);

  const emptyDescription = computed(() => {
    if (loading.value) {
      return '正在加载商品库';
    }
    return '当前筛选条件下暂无商品';
  });

  let unlistenC2CItemsChanged: (() => void) | null = null;
  let pendingRefreshTimer: ReturnType<typeof setTimeout> | null = null;
  let hasActivatedOnce = false;

  function resolveQuery(firstPage: boolean) {
    const page = firstPage ? 1 : pagination.value.page;
    const startTime = timeRangeEnable.value && timeRange.value ? timeRange.value[0] : -1;
    const endTime = timeRangeEnable.value && timeRange.value ? timeRange.value[1] : -1;
    const fromPrice = priceRangeEnable.value ? Number(priceRange.value[0] ?? -1) : -1;
    const toPrice = priceRangeEnable.value ? Number(priceRange.value[1] ?? -1) : -1;

    return {
      page,
      startTime,
      endTime,
      fromPrice,
      toPrice
    };
  }

  function clearRealtimeRefreshTimer() {
    if (pendingRefreshTimer) {
      clearTimeout(pendingRefreshTimer);
      pendingRefreshTimer = null;
    }
  }

  function queueRealtimeRefresh() {
    if (pendingRefreshTimer) {
      return;
    }
    pendingRefreshTimer = setTimeout(() => {
      pendingRefreshTimer = null;
      search(false);
    }, REALTIME_REFRESH_DEBOUNCE_MS);
  }

  function ensureRealtimeSubscription() {
    if (unlistenC2CItemsChanged) {
      return;
    }
    unlistenC2CItemsChanged = EventsOn('c2c_items_changed', _ => {
      queueRealtimeRefresh();
    });
  }

  function removeRealtimeSubscription() {
    if (unlistenC2CItemsChanged) {
      unlistenC2CItemsChanged();
      unlistenC2CItemsChanged = null;
    }
    clearRealtimeRefreshTimer();
  }

  function goDetail(item: catalog.C2CItemGroupVO) {
    router.push(`/home/${item.skuId}`);
  }

  function search(firstPage: boolean = false) {
    loading.value = true;
    const query = resolveQuery(firstPage);

    fetchCatalogList({
      page: query.page,
      pageSize: pagination.value.pageSize,
      keyword: searchText.value,
      sortOption: sortOpt.value,
      startTime: query.startTime,
      endTime: query.endTime,
      fromPrice: query.fromPrice,
      toPrice: query.toPrice
    })
      .then(result => {
        pagination.value.page = query.page;
        pagination.value.pageCount = result.totalPages;
        pagination.value.itemCount = result.total;
        data.value = result.items;
      })
      .catch(err => {
        message.error(err?.message || '请求失败');
      })
      .finally(() => {
        loading.value = false;
      });
  }

  onMounted(() => {
    ensureRealtimeSubscription();
    search();
  });

  onActivated(() => {
    ensureRealtimeSubscription();
    if (hasActivatedOnce) {
      search();
      return;
    }
    hasActivatedOnce = true;
  });

  onDeactivated(() => {
    removeRealtimeSubscription();
  });

  onUnmounted(() => {
    removeRealtimeSubscription();
  });

  return {
    loading,
    searchText,
    timeRange,
    timeRangeEnable,
    priceRangeEnable,
    priceRange,
    sortOpt,
    sortways,
    data,
    pagination,
    emptyDescription,
    goDetail,
    search
  };
}
