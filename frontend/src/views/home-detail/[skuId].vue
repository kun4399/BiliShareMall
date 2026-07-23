<script setup lang="ts">
import dayjs from 'dayjs';
import { ArrowBack, Search } from '@vicons/ionicons5';
import { fallbackToImageProxy, normalizeImage } from '@/features/catalog/shared';
import { useCatalogDetail } from '@/features/catalog/useCatalogDetail';
import { useAppStore } from '@/store/modules/app';

const appStore = useAppStore();
const {
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
  getStatusType,
  handleCopy,
  goBack,
  search
} = useCatalogDetail();

function formatFirstSeenTime(value: number) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm') : '-';
}
</script>

<template>
  <main class="detail-page">
    <NButton tertiary class="detail-page__back" aria-label="返回商品数据库" @click="goBack">
      <template #icon><ArrowBack /></template>
      返回数据库
    </NButton>

    <section class="detail-hero">
      <div class="detail-hero__image-shell">
        <img
          v-if="detail?.detailImg && normalizeImage(detail.detailImg)"
          :src="normalizeImage(detail.detailImg)"
          :alt="detail.c2cItemsName"
          width="360"
          height="360"
          decoding="async"
          referrerpolicy="no-referrer"
          class="detail-hero__image"
          @error="event => fallbackToImageProxy(event, detail?.detailImg || '')"
        />
        <div v-else class="detail-hero__fallback">
          <SvgIcon icon="mdi:image-filter-hdr" class="text-34px text-#94a3b8" />
        </div>
      </div>

      <div class="detail-hero__content">
        <p class="detail-hero__eyebrow">SKU #{{ skuId }}</p>
        <h1 class="detail-hero__title">{{ detail?.c2cItemsName || '商品详情' }}</h1>
        <p class="detail-hero__desc">
          展示当前 SKU 下已采集的发布商品；记录会立即显示，商品状态在后台自动校验更新。
        </p>
        <div class="detail-hero__metrics">
          <span>已收录 {{ pagination.itemCount.toLocaleString() }} 条发布</span>
          <span>后台状态校验已启用</span>
        </div>
      </div>
    </section>

    <NCard title="筛选与排序" class="detail-filter-card" :bordered="false">
      <template #header-extra>
        <NButton type="primary" @click="search(true)">
          <template #icon><Search /></template>
          刷新
        </NButton>
      </template>

      <NFlex class="detail-filter-row" :wrap="true">
        <NSelect
          v-model:value="sortOpt"
          :options="sortways"
          label-field="label"
          value-field="value"
          class="detail-filter-control"
          aria-label="商品排序"
          @update:value="() => search(true)"
        />
        <NSelect
          v-model:value="statusFilter"
          :options="statusOptions"
          class="detail-filter-control detail-filter-control--status"
          aria-label="状态筛选"
          @update:value="() => search(true)"
        />
      </NFlex>
    </NCard>

    <NCard title="发布商品" class="detail-table-card" :bordered="false">
      <template #header-extra>
        <span class="detail-table-card__summary">共 {{ pagination.itemCount.toLocaleString() }} 条</span>
      </template>

      <NDataTable
        v-if="!appStore.isMobile"
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

      <div v-else class="detail-mobile-list" :class="{ 'is-loading': loading }">
        <article v-for="item in items" :key="item.c2cItemsId" class="detail-mobile-card">
          <div class="detail-mobile-card__topline">
            <strong>{{ item.showPrice ? `${item.showPrice} 元` : `${item.price.toFixed(2)} 元` }}</strong>
            <NTag :type="getStatusType(item.status)" size="small" round :bordered="false">
              {{ item.status || '未知' }}
            </NTag>
          </div>

          <dl class="detail-mobile-card__details">
            <div>
              <dt>卖家</dt>
              <dd>{{ item.sellerName || '-' }}</dd>
            </div>
            <div>
              <dt>用户 ID</dt>
              <dd>{{ item.sellerUID || '-' }}</dd>
            </div>
            <div>
              <dt>首次抓取</dt>
              <dd>{{ formatFirstSeenTime(item.firstSeenTime) }}</dd>
            </div>
          </dl>

          <div class="detail-mobile-card__actions">
            <NButton size="small" secondary @click="handleCopy(item.link)">复制链接</NButton>
            <NButton
              size="small"
              type="primary"
              tag="a"
              :href="item.link"
              target="_blank"
              rel="noopener noreferrer"
            >
              打开商品
            </NButton>
          </div>
        </article>

        <NEmpty v-if="!items.length && !loading" description="当前条件下暂无发布商品" />
        <NPagination
          v-model:page="pagination.page"
          :page-count="pagination.pageCount"
          :page-slot="5"
          class="detail-mobile-pagination"
          @update:page="() => search()"
        />
      </div>
    </NCard>
  </main>
</template>

<style scoped>
.detail-page {
  display: flex;
  flex-direction: column;
  gap: var(--bsm-space-4);
  width: min(100%, 1680px);
  margin: 0 auto;
}

.detail-page__back {
  align-self: flex-start;
}

.detail-hero {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  gap: 22px;
  padding: 22px;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  background:
    radial-gradient(circle at 92% 12%, rgba(251, 114, 153, 0.18), transparent 34%),
    var(--bsm-surface);
  box-shadow: var(--bsm-shadow-sm);
}

.detail-hero__image-shell {
  overflow: hidden;
  border-radius: var(--bsm-radius-lg);
  aspect-ratio: 1 / 1;
  background: var(--bsm-surface-muted);
}

.detail-hero__image {
  display: block;
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
  min-width: 0;
  flex-direction: column;
  justify-content: center;
  gap: 10px;
}

.detail-hero__eyebrow {
  margin: 0;
  color: var(--bsm-primary);
  font-size: 12px;
  font-weight: 750;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.detail-hero__title {
  margin: 0;
  color: var(--bsm-text);
  font-size: clamp(24px, 3vw, 34px);
  line-height: 1.25;
  letter-spacing: -0.025em;
}

.detail-hero__desc {
  max-width: 760px;
  margin: 0;
  color: var(--bsm-text-muted);
  line-height: 1.7;
}

.detail-hero__metrics {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.detail-hero__metrics span {
  padding: 5px 9px;
  border: 1px solid var(--bsm-border);
  border-radius: 999px;
  color: var(--bsm-text-muted);
  background: color-mix(in srgb, var(--bsm-surface) 86%, transparent);
  font-size: 11px;
}

.detail-filter-card,
.detail-table-card {
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  box-shadow: var(--bsm-shadow-xs);
}

.detail-filter-row {
  gap: 12px;
}

.detail-filter-control {
  min-width: 220px;
  flex: 1 1 260px;
}

.detail-filter-control--status {
  max-width: 320px;
}

.detail-table-card__summary {
  color: var(--bsm-text-muted);
  font-size: 13px;
}

.detail-mobile-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: opacity 0.2s ease;
}

.detail-mobile-list.is-loading {
  opacity: 0.6;
}

.detail-mobile-card {
  padding: 14px;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-lg);
  background: var(--bsm-surface-muted);
}

.detail-mobile-card__topline,
.detail-mobile-card__actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.detail-mobile-card__topline strong {
  color: var(--bsm-text);
  font-size: 18px;
}

.detail-mobile-card__details {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin: 14px 0;
}

.detail-mobile-card__details div:last-child {
  grid-column: 1 / -1;
}

.detail-mobile-card__details dt {
  margin-bottom: 3px;
  color: var(--bsm-text-muted);
  font-size: 11px;
}

.detail-mobile-card__details dd {
  overflow: hidden;
  margin: 0;
  color: var(--bsm-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-mobile-card__actions > * {
  flex: 1;
}

.detail-mobile-pagination {
  align-self: center;
  margin-top: 8px;
}

@media (max-width: 768px) {
  .detail-hero {
    grid-template-columns: 94px minmax(0, 1fr);
    gap: 14px;
    padding: 14px;
    border-radius: var(--bsm-radius-lg);
  }

  .detail-hero__title {
    font-size: 20px;
  }

  .detail-hero__desc,
  .detail-hero__metrics {
    display: none;
  }

  .detail-filter-card :deep(.n-card-header) {
    align-items: flex-start;
    flex-direction: column;
    gap: 10px;
  }

  .detail-filter-card :deep(.n-card-header__extra) {
    width: 100%;
    margin-left: 0;
  }

  .detail-filter-card :deep(.n-card-header__extra .n-button),
  .detail-filter-control,
  .detail-filter-control--status {
    width: 100%;
    max-width: none;
    min-width: 0;
  }
}
</style>
