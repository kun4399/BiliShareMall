import { computed, onActivated, onBeforeUnmount, onMounted, ref } from 'vue';
import { useMessage } from 'naive-ui';
import { useRouter } from 'vue-router';
import type { catalog } from '~/wailsjs/go/models';
import { OnAppEvent } from '@/gateway';
import { fetchCatalogList, type CatalogListQuery } from './api';
import {
  CatalogRequestCoordinator,
  catalogPageCache,
  createCatalogQueryKey,
  getAdjacentCatalogPages
} from './page-cache';

interface SortWay {
  value: number;
  label: string;
}

const cacheFreshnessMs = 30_000;

export function useCatalogList() {
  const router = useRouter();
  const message = useMessage();

  const loading = ref(false);
  const refreshing = ref(false);
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
    { value: 2, label: '参考价升序' },
    { value: 3, label: '参考价降序' }
  ]);

  const emptyDescription = computed(() => (loading.value ? '正在加载商品库' : '当前筛选条件下暂无商品'));
  const initialLoading = computed(() => loading.value && data.value.length === 0);
  const activeFilterCount = computed(() => Number(timeRangeEnable.value) + Number(priceRangeEnable.value));

  let hasActivatedOnce = false;
  const requestCoordinator = new CatalogRequestCoordinator();
  let activeController: AbortController | null = null;
  let refreshTimer: ReturnType<typeof setTimeout> | null = null;
  let unsubscribeItemsChanged: (() => void) | null = null;
  const prefetching = new Map<string, Promise<catalog.C2CItemGroupListVO>>();

  function resolveQuery(firstPage: boolean, pageOverride?: number): CatalogListQuery {
    const page = pageOverride ?? (firstPage ? 1 : pagination.value.page);
    const startTime = timeRangeEnable.value && timeRange.value ? timeRange.value[0] : -1;
    const endTime = timeRangeEnable.value && timeRange.value ? timeRange.value[1] : -1;
    const fromPrice = priceRangeEnable.value ? Number(priceRange.value[0] ?? -1) : -1;
    const toPrice = priceRangeEnable.value ? Number(priceRange.value[1] ?? -1) : -1;

    return {
      page,
      pageSize: pagination.value.pageSize,
      keyword: searchText.value,
      sortOption: sortOpt.value,
      startTime,
      endTime,
      fromPrice,
      toPrice
    };
  }

  function applyResult(result: catalog.C2CItemGroupListVO, requestedPage: number) {
    pagination.value.page = requestedPage;
    pagination.value.pageCount = result.totalPages;
    pagination.value.itemCount = result.total;
    data.value = result.items;
  }

  function goDetail(item: catalog.C2CItemGroupVO) {
    router.push(`/home/${item.skuId}`);
  }

  function prefetch(query: CatalogListQuery) {
    if (query.page < 1 || query.page > pagination.value.pageCount) return;
    const key = createCatalogQueryKey(query);
    if (catalogPageCache.get(key) || prefetching.has(key)) return;

    const token = requestCoordinator.snapshot();
    const promise = fetchCatalogList(query)
      .then(result => {
        if (requestCoordinator.isCurrent(token)) {
          catalogPageCache.set(key, result);
        }
        return result;
      })
      .finally(() => {
        if (prefetching.get(key) === promise) {
          prefetching.delete(key);
        }
      });
    prefetching.set(key, promise);
  }

  function prefetchAdjacent(query: CatalogListQuery) {
    for (const page of getAdjacentCatalogPages(query.page, pagination.value.pageCount)) {
      prefetch({ ...query, page });
    }
  }

  async function search(
    firstPage: boolean = false,
    options: { force?: boolean; silent?: boolean } = {}
  ) {
    const query = resolveQuery(firstPage);
    const key = createCatalogQueryKey(query);
    const cached = options.force ? null : catalogPageCache.get(key);
    const requestToken = requestCoordinator.start();
    activeController?.abort();
    activeController = null;

    if (cached) {
      applyResult(cached.value, query.page);
      prefetchAdjacent(query);
      if (Date.now() - cached.cachedAt < cacheFreshnessMs) {
        loading.value = false;
        refreshing.value = false;
        return;
      }
      options = { ...options, silent: true };
    }

    activeController = typeof AbortController === 'undefined' ? null : new AbortController();

    if (options.silent || data.value.length > 0) {
      refreshing.value = true;
    } else {
      loading.value = true;
    }

    try {
      const pendingPrefetch = prefetching.get(key);
      const result = pendingPrefetch || (await fetchCatalogList(query, activeController?.signal));
      const resolved = await result;
      if (!requestCoordinator.isCurrent(requestToken)) return;
      catalogPageCache.set(key, resolved);
      applyResult(resolved, query.page);
      prefetchAdjacent(query);
    } catch (err: any) {
      if (err?.name !== 'AbortError' && requestCoordinator.isCurrent(requestToken)) {
        message.error(err?.message || '请求失败');
      }
    } finally {
      if (requestCoordinator.isCurrent(requestToken)) {
        loading.value = false;
        refreshing.value = false;
      }
    }
  }

  function refresh() {
    return search(false, { force: true });
  }

  function resetFilters() {
    timeRangeEnable.value = false;
    priceRangeEnable.value = false;
    priceRange.value = [0, 9999];
    sortOpt.value = 1;
    void search(true);
  }

  function scheduleEventRefresh() {
    requestCoordinator.invalidate();
    catalogPageCache.clear();
    prefetching.clear();
    if (refreshTimer) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      refreshTimer = null;
      void search(false, { force: true, silent: true });
    }, 350);
  }

  onMounted(() => {
    unsubscribeItemsChanged = OnAppEvent('c2c_items_changed', scheduleEventRefresh);
    void search();
  });

  onActivated(() => {
    if (hasActivatedOnce) {
      void search(false, { silent: true });
      return;
    }
    hasActivatedOnce = true;
  });

  onBeforeUnmount(() => {
    activeController?.abort();
    if (refreshTimer) clearTimeout(refreshTimer);
    unsubscribeItemsChanged?.();
  });

  return {
    loading,
    refreshing,
    initialLoading,
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
    activeFilterCount,
    goDetail,
    search,
    refresh,
    resetFilters
  };
}
