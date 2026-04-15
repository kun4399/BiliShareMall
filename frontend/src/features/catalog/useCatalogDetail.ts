import dayjs from 'dayjs';
import { NButton, NButtonGroup, NTag, useMessage } from 'naive-ui';
import { computed, h, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { catalog } from '~/wailsjs/go/models';
import { getToken } from '@/store/modules/auth/shared';
import { copyText } from '@/utils/clipboard';
import { fetchCatalogDetail } from './api';

interface SortWay {
  value: number;
  label: string;
}

export function useCatalogDetail() {
  const route = useRoute();
  const router = useRouter();
  const message = useMessage();
  const loading = ref(false);
  const detail = ref<catalog.C2CItemDetailListVO | null>(null);
  const sortOpt = ref(1);
  const statusFilter = ref('');
  const pagination = ref({
    page: 1,
    pageCount: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: true,
    pageSizes: [10, 20, 50]
  });

  const sortways = ref<SortWay[]>([
    { value: 1, label: '首次抓取时间降序' },
    { value: 2, label: '首次抓取时间升序' },
    { value: 3, label: '价格升序' },
    { value: 4, label: '价格降序' }
  ]);

  const statusOptions = [
    { label: '全部状态', value: '' },
    { label: '在售', value: '在售' },
    { label: '下架', value: '下架' },
    { label: '已售出', value: '已售出' }
  ];

  const skuId = computed(() => Number(route.params.skuId || 0));
  const items = computed(() => detail.value?.items ?? []);

  function getStatusType(status: string) {
    if (status === '在售') {
      return 'success';
    }
    if (status === '已售出') {
      return 'warning';
    }
    return 'default';
  }

  async function handleCopy(link: string) {
    const copied = await copyText(link);
    if (!copied) {
      message.error(`复制失败，请自行复制链接：${link}`);
      return;
    }
    message.success('链接已复制');
  }

  const columns = [
    {
      title: '价格',
      key: 'showPrice',
      width: 110,
      render(row: catalog.C2CItemDetailVO) {
        return row.showPrice ? `${row.showPrice} 元` : `${row.price.toFixed(2)} 元`;
      }
    },
    {
      title: '用户名',
      key: 'sellerName',
      ellipsis: {
        tooltip: true
      }
    },
    {
      title: '用户 ID',
      key: 'sellerUID',
      ellipsis: {
        tooltip: true
      }
    },
    {
      title: '首次抓取时间',
      key: 'firstSeenTime',
      width: 180,
      render(row: catalog.C2CItemDetailVO) {
        if (!row.firstSeenTime) {
          return '-';
        }
        return dayjs(row.firstSeenTime).format('YYYY-MM-DD HH:mm');
      }
    },
    {
      title: '状态',
      key: 'status',
      width: 100,
      render(row: catalog.C2CItemDetailVO) {
        return h(
          NTag,
          {
            bordered: false,
            type: getStatusType(row.status)
          },
          { default: () => row.status || '-' }
        );
      }
    },
    {
      title: '链接',
      key: 'link',
      width: 160,
      render(row: catalog.C2CItemDetailVO) {
        return h(
          NButtonGroup,
          {},
          {
            default: () => [
              h(
                NButton,
                {
                  size: 'small',
                  onClick: () => handleCopy(row.link)
                },
                { default: () => '复制' }
              ),
              h(
                NButton,
                {
                  size: 'small',
                  tertiary: true,
                  onClick: () => window.open(row.link, '_blank', 'noopener,noreferrer')
                },
                { default: () => '打开' }
              )
            ]
          }
        );
      }
    }
  ];

  function goBack() {
    router.push('/home');
  }

  function search(firstPage: boolean = false) {
    if (!skuId.value) {
      message.error('缺少 skuId');
      return;
    }

    loading.value = true;
    const page = firstPage ? 1 : pagination.value.page;

    fetchCatalogDetail({
      skuId: skuId.value,
      page,
      pageSize: pagination.value.pageSize,
      sortOption: sortOpt.value,
      statusFilter: statusFilter.value,
      cookie: getToken()
    })
      .then(result => {
        detail.value = result;
        pagination.value.page = page;
        pagination.value.pageCount = result.totalPages;
        pagination.value.itemCount = result.total;
      })
      .catch(err => {
        message.error(err?.message || '详情加载失败');
      })
      .finally(() => {
        loading.value = false;
      });
  }

  onMounted(() => {
    search();
  });

  return {
    loading,
    detail,
    sortOpt,
    sortways,
    statusFilter,
    statusOptions,
    pagination,
    columns,
    items,
    skuId,
    goBack,
    search
  };
}
