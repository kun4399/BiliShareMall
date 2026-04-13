<script setup lang="ts">
import { ArrowBack, Search } from '@vicons/ionicons5';
import { normalizeImage } from '@/features/catalog/shared';
import { useCatalogDetail } from '@/features/catalog/useCatalogDetail';

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
  goBack,
  search
} = useCatalogDetail();
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
