<script setup lang="ts">
import { Search } from '@vicons/ionicons5';
import { normalizeImage } from '@/features/catalog/shared';
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
        <NSpace size="large">
        <NInput v-model:value="searchText" clearable :placeholder="$t('common.keywordSearch')" @keyup.enter="search(true)">
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
            <p class="catalog-card__meta">{{ item.referencePriceLabel || '参考价待补充' }}</p>
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
}

.filter-card {
  border-radius: 20px;
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
  border-radius: 24px;
  padding: 14px;
  text-align: left;
  background:
    radial-gradient(circle at top left, rgba(14, 165, 233, 0.14), transparent 42%),
    linear-gradient(180deg, #fffdf8 0%, #ffffff 100%);
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.08);
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
  border-radius: 18px;
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
  color: #0f172a;
}

.catalog-card__meta {
  margin: 0;
  font-size: 12px;
  line-height: 1.4;
  color: #64748b;
}

.catalog-empty {
  padding: 48px 0;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.9);
}

.catalog-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 0 4px 12px;
}

.catalog-footer__summary {
  color: #475569;
  font-size: 13px;
}

@media (max-width: 768px) {
  .catalog-footer {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
