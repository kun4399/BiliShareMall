<script setup lang="ts">
import { ArrowBack, Search } from '@vicons/ionicons5';
import dayjs from 'dayjs';
import { useClipboard } from '@vueuse/core';
import { NButton, NButtonGroup, NTag, useMessage } from 'naive-ui';
import { computed, h, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ListC2CItemDetailBySku } from '~/wailsjs/go/app/App';
import type { app } from '~/wailsjs/go/models';
import { getToken } from '@/store/modules/auth/shared';

interface SortWay {
  value: number;
  label: string;
}

const route = useRoute();
const router = useRouter();
const message = useMessage();
const loading = ref(false);
const detail = ref<app.C2CItemDetailListVO | null>(null);
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
  { value: 1, label: '时间降序' },
  { value: 2, label: '时间升序' },
  { value: 3, label: '价格升序' },
  { value: 4, label: '价格降序' }
]);

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '在售', value: '在售' },
  { label: '下架', value: '下架' },
  { label: '已售出', value: '已售出' }
];

const { copy, isSupported } = useClipboard();

const skuId = computed(() => Number(route.params.skuId || 0));
const items = computed(() => detail.value?.items ?? []);

const columns = [
  {
    title: '价格',
    key: 'showPrice',
    width: 110,
    render(row: app.C2CItemDetailVO) {
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
    title: '创建时间',
    key: 'publishTime',
    width: 180,
    render(row: app.C2CItemDetailVO) {
      if (!row.publishTime) {
        return '-';
      }
      return dayjs(row.publishTime).format('YYYY-MM-DD HH:mm');
    }
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render(row: app.C2CItemDetailVO) {
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
    render(row: app.C2CItemDetailVO) {
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

function normalizeImage(url: string) {
  if (!url) {
    return '';
  }
  if (url.startsWith('//')) {
    return `https:${url}`;
  }
  return url;
}

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
  if (!isSupported) {
    message.error(`复制失败，请自行复制链接：${link}`);
    return;
  }
  await copy(link);
  message.success('链接已复制');
}

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

  ListC2CItemDetailBySku(
    skuId.value,
    page,
    pagination.value.pageSize,
    sortOpt.value,
    statusFilter.value,
    getToken()
  )
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
</script>

<template>
  <div class="detail-page">
    <NButton tertiary class="detail-page__back" @click="goBack">
      <template #icon>
        <ArrowBack />
      </template>
      返回数据库
    </NButton>

    <div class="detail-hero">
      <div class="detail-hero__image-shell">
        <img
          v-if="detail?.detailImg && normalizeImage(detail.detailImg)"
          :src="normalizeImage(detail.detailImg)"
          :alt="detail.c2cItemsName"
          class="detail-hero__image"
        />
        <div v-else class="detail-hero__fallback">
          <SvgIcon icon="mdi:image-filter-hdr" class="text-34px text-#94a3b8" />
        </div>
      </div>

      <div class="detail-hero__content">
        <p class="detail-hero__eyebrow">SKU #{{ skuId }}</p>
        <h1 class="detail-hero__title">{{ detail?.c2cItemsName || '商品详情' }}</h1>
        <p class="detail-hero__desc">这里展示当前 skuId 下已采集到的所有发布商品，并在打开页面时实时校验状态。</p>
      </div>
    </div>

    <NCard title="筛选与排序" class="detail-filter-card">
      <template #header-extra>
        <NButton type="primary" @click="search(true)">
          <template #icon>
            <Search />
          </template>
          刷新
        </NButton>
      </template>

      <NFlex class="detail-filter-row" :wrap="true">
        <NSelect
          v-model:value="sortOpt"
          :options="sortways"
          label-field="label"
          value-field="value"
          class="min-w-220px"
          @update:value="() => search(true)"
        />
        <NSelect
          v-model:value="statusFilter"
          :options="statusOptions"
          class="min-w-180px"
          @update:value="() => search(true)"
        />
      </NFlex>
    </NCard>

    <NCard title="发布商品" class="detail-table-card">
      <template #header-extra>
        <span class="detail-table-card__summary">共 {{ pagination.itemCount }} 条</span>
      </template>

      <NDataTable
        remote
        :data="items"
        :columns="columns"
        :loading="loading"
        :pagination="pagination"
        @update:page="
          page => {
            pagination.page = page;
            search();
          }
        "
        @update:page-size="
          pageSize => {
            pagination.pageSize = pageSize;
            search(true);
          }
        "
      />
    </NCard>
  </div>
</template>

<style scoped>
.detail-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.detail-page__back {
  align-self: flex-start;
}

.detail-hero {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  gap: 20px;
  padding: 20px;
  border-radius: 28px;
  background:
    radial-gradient(circle at top right, rgba(249, 115, 22, 0.18), transparent 36%),
    linear-gradient(135deg, #fff7ed 0%, #ffffff 54%, #f0f9ff 100%);
  box-shadow: 0 18px 38px rgba(15, 23, 42, 0.08);
}

.detail-hero__image-shell {
  overflow: hidden;
  border-radius: 24px;
  aspect-ratio: 1 / 1;
  background: rgba(255, 255, 255, 0.72);
}

.detail-hero__image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.detail-hero__fallback {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
}

.detail-hero__content {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 10px;
}

.detail-hero__eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #ea580c;
}

.detail-hero__title {
  margin: 0;
  font-size: 28px;
  line-height: 1.25;
  color: #0f172a;
}

.detail-hero__desc {
  margin: 0;
  color: #475569;
  line-height: 1.7;
}

.detail-filter-card,
.detail-table-card {
  border-radius: 20px;
}

.detail-filter-row {
  gap: 12px;
}

.detail-table-card__summary {
  color: #64748b;
  font-size: 13px;
}

@media (max-width: 768px) {
  .detail-hero {
    grid-template-columns: 1fr;
  }
}
</style>
