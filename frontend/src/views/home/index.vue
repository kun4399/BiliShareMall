<script setup lang="ts">
import { Search } from '@vicons/ionicons5';
import { normalizeImage, resolveReferencePriceLabel } from '@/features/catalog/shared';
import { useCatalogList } from '@/features/catalog/useCatalogList';

const {
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
  search,
  refresh
} = useCatalogList();
</script>

<template>
  <div class="catalog-page">
    <NCard class="filter-card" title="数据库">
      <template #header-extra>
        <NSpace class="catalog-actions" size="medium" :wrap="true">
          <NInput
            v-model:value="searchText"
            class="catalog-actions__search"
            clearable
            :placeholder="$t('common.keywordSearch')"
            @keyup.enter="search(true)"
          >
            <template #prefix>
              <icon-uil-search class="text-15px text-#c2c2c2" />
            </template>
          </NInput>
          <NButton type="primary" @click="search(true)">
            <template #icon>
              <Search />
            </template>
            搜索
          </NButton>
          <NButton @click="refresh">
            <template #icon>
              <icon-mdi-refresh />
            </template>
            刷新
          </NButton>
        </NSpace>
      </template>

      <NCollapse default-expanded-names="sort">
        <NCollapseItem title="发布时间">
          <NDatePicker v-model:value="timeRange" type="datetimerange" clearable />
          <template #header-extra>
            <NSwitch v-model:value="timeRangeEnable" />
          </template>
        </NCollapseItem>

        <NCollapseItem title="参考价格区间">
          <NFlex>
            <NInputNumber v-model:value="priceRange[0]" :precision="2">
              <template #suffix>元</template>
            </NInputNumber>
            <NInputNumber v-model:value="priceRange[1]" :precision="2">
              <template #suffix>元</template>
            </NInputNumber>
          </NFlex>
          <template #header-extra>
            <NSwitch v-model:value="priceRangeEnable" />
          </template>
        </NCollapseItem>

        <NCollapseItem title="排序" name="sort">
          <NRadioGroup v-model:value="sortOpt" name="catalogSort">
            <NRadioButton
              v-for="sortway in sortways"
              :key="sortway.value"
              :value="sortway.value"
              :label="sortway.label"
            />
          </NRadioGroup>
        </NCollapseItem>
      </NCollapse>
    </NCard>

    <NSpin :show="loading">
      <div v-if="data.length" class="catalog-grid">
        <button
          v-for="item in data"
          :key="item.skuId"
          type="button"
          class="catalog-card"
          @click="goDetail(item)"
        >
          <div class="catalog-card__image-shell">
            <img
              v-if="normalizeImage(item.detailImg)"
              :src="normalizeImage(item.detailImg)"
              :alt="item.c2cItemsName"
              class="catalog-card__image"
            />
            <div v-else class="catalog-card__fallback">
              <SvgIcon icon="mdi:image-off-outline" class="text-26px text-#8f9aa8" />
            </div>
          </div>

          <div class="catalog-card__content">
            <p class="catalog-card__title">{{ item.c2cItemsName }}</p>
            <p class="catalog-card__meta">
              {{ resolveReferencePriceLabel(item.referencePriceLabel, item.referencePriceMin, item.referencePriceMax) }}
            </p>
          </div>
        </button>
      </div>

      <NEmpty v-else :description="emptyDescription" class="catalog-empty" />
    </NSpin>

    <div class="catalog-footer">
      <span class="catalog-footer__summary">共 {{ pagination.itemCount }} 个聚合商品</span>
      <NPagination
        v-model:page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :item-count="pagination.itemCount"
        :page-count="pagination.pageCount"
        :page-sizes="[12, 24, 36]"
        show-size-picker
        @update:page="() => search()"
        @update:page-size="() => search(true)"
      />
    </div>
  </div>
</template>

<style scoped>
.catalog-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  width: min(100%, 1680px);
  margin: 0 auto;
}

.filter-card {
  border-radius: 8px;
}

.catalog-actions__search {
  width: min(320px, 32vw);
}

.catalog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 18px;
}

.catalog-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 8px;
  padding: 14px;
  text-align: left;
  background: rgb(var(--container-bg-color));
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.07);
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    border-color 0.2s ease;
}

.catalog-card:hover {
  transform: translateY(-4px);
  border-color: rgba(14, 165, 233, 0.4);
  box-shadow: 0 18px 42px rgba(14, 165, 233, 0.16);
}

.catalog-card__image-shell {
  position: relative;
  overflow: hidden;
  border-radius: 6px;
  aspect-ratio: 1 / 1;
  background:
    linear-gradient(135deg, rgba(2, 132, 199, 0.12), rgba(249, 115, 22, 0.16)),
    #f8fafc;
}

.catalog-card__image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.catalog-card__fallback {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
}

.catalog-card__content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.catalog-card__title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.5;
  color: rgb(var(--base-text-color));
}

.catalog-card__meta {
  margin: 0;
  font-size: 12px;
  line-height: 1.4;
  color: color-mix(in srgb, rgb(var(--base-text-color)) 68%, transparent);
}

.catalog-empty {
  padding: 48px 0;
}

.catalog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 0 4px 12px;
}

.catalog-footer__summary {
  color: color-mix(in srgb, rgb(var(--base-text-color)) 72%, transparent);
  font-size: 13px;
}

@media (max-width: 768px) {
  .catalog-page {
    gap: 12px;
  }

  .filter-card :deep(.n-card-header) {
    align-items: stretch;
    flex-direction: column;
    gap: 12px;
  }

  .filter-card :deep(.n-card-header__extra) {
    margin-left: 0;
    width: 100%;
  }

  .catalog-actions,
  .catalog-actions__search {
    width: 100%;
  }

  .catalog-actions :deep(> div:first-child) {
    flex: 0 0 100%;
    width: 100%;
  }

  .catalog-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .catalog-card {
    gap: 10px;
    padding: 10px;
  }

  .catalog-footer {
    flex-direction: column;
    align-items: stretch;
  }
}

@media (max-width: 420px) {
  .catalog-grid {
    grid-template-columns: 1fr;
  }
}
</style>
