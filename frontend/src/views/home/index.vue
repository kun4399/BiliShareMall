<script setup lang="ts">
import { computed, ref } from 'vue';
import { Search } from '@vicons/ionicons5';
import { fallbackToImageProxy, normalizeImage, resolveReferencePriceLabel } from '@/features/catalog/shared';
import { useCatalogList } from '@/features/catalog/useCatalogList';
import { useAppStore } from '@/store/modules/app';

const appStore = useAppStore();
const showMobileFilters = ref(false);

const {
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
} = useCatalogList();

const resultSummary = computed(() => {
  if (!pagination.value.itemCount) return '等待发现商品';
  return `${pagination.value.itemCount.toLocaleString()} 个聚合商品`;
});

function applyMobileFilters() {
  showMobileFilters.value = false;
  void search(true);
}
</script>

<template>
  <main class="catalog-page">
    <section class="catalog-hero">
      <div>
        <p class="catalog-hero__eyebrow">MARKET DATABASE</p>
        <h1 class="catalog-hero__title">商品数据库</h1>
        <p class="catalog-hero__description">快速检索已采集的市集商品，筛选条件与翻页结果会在当前会话中保留。</p>
      </div>
      <div class="catalog-hero__summary">
        <span class="catalog-hero__summary-label">当前收录</span>
        <strong>{{ resultSummary }}</strong>
      </div>
    </section>

    <section class="catalog-toolbar">
      <NInput
        v-model:value="searchText"
        class="catalog-toolbar__search"
        clearable
        :placeholder="$t('common.keywordSearch')"
        aria-label="搜索商品名称"
        @keyup.enter="search(true)"
      >
        <template #prefix>
          <icon-uil-search class="text-16px text-#94a3b8" />
        </template>
      </NInput>

      <NButton type="primary" class="catalog-toolbar__primary" @click="search(true)">
        <template #icon><Search /></template>
        搜索
      </NButton>
      <NButton v-if="appStore.isMobile" @click="showMobileFilters = true">
        <template #icon><icon-mdi-filter-variant /></template>
        筛选
        <NTag v-if="activeFilterCount" size="tiny" round type="info">{{ activeFilterCount }}</NTag>
      </NButton>
      <NButton :loading="refreshing" secondary @click="refresh">
        <template #icon><icon-mdi-refresh /></template>
        刷新
      </NButton>
    </section>

    <section v-if="!appStore.isMobile" class="catalog-filter-panel">
      <div class="catalog-filter-panel__header">
        <div>
          <h2>筛选与排序</h2>
          <p>启用所需条件后点击搜索，未启用的输入不会参与查询。</p>
        </div>
        <NButton v-if="activeFilterCount" text type="primary" @click="resetFilters">清除筛选</NButton>
      </div>

      <div class="catalog-filter-grid">
        <div class="catalog-filter-field">
          <div class="catalog-filter-field__label">
            <span>发布时间</span>
            <NSwitch v-model:value="timeRangeEnable" size="small" />
          </div>
          <NDatePicker
            v-model:value="timeRange"
            type="datetimerange"
            clearable
            :disabled="!timeRangeEnable"
            class="catalog-filter-control"
          />
        </div>

        <div class="catalog-filter-field">
          <div class="catalog-filter-field__label">
            <span>参考价格</span>
            <NSwitch v-model:value="priceRangeEnable" size="small" />
          </div>
          <NInputGroup>
            <NInputNumber v-model:value="priceRange[0]" :precision="2" :disabled="!priceRangeEnable">
              <template #suffix>元</template>
            </NInputNumber>
            <NInputNumber v-model:value="priceRange[1]" :precision="2" :disabled="!priceRangeEnable">
              <template #suffix>元</template>
            </NInputNumber>
          </NInputGroup>
        </div>

        <div class="catalog-filter-field">
          <div class="catalog-filter-field__label"><span>排序方式</span></div>
          <NRadioGroup v-model:value="sortOpt" name="catalogSort" class="catalog-sort-group">
            <NRadioButton
              v-for="sortway in sortways"
              :key="sortway.value"
              :value="sortway.value"
              :label="sortway.label"
            />
          </NRadioGroup>
        </div>
      </div>
    </section>

    <div v-if="initialLoading" class="catalog-grid" aria-label="正在加载商品">
      <article v-for="index in pagination.pageSize" :key="index" class="catalog-card catalog-card--skeleton">
        <NSkeleton height="auto" class="catalog-card__skeleton-image" />
        <NSkeleton text :repeat="2" />
      </article>
    </div>

    <div v-else-if="data.length" class="catalog-grid" :class="{ 'is-refreshing': refreshing }">
      <button
        v-for="item in data"
        :key="item.skuId"
        type="button"
        class="catalog-card"
        :aria-label="`查看 ${item.c2cItemsName} 的商品详情`"
        @click="goDetail(item)"
      >
        <div class="catalog-card__image-shell">
          <img
            v-if="normalizeImage(item.detailImg)"
            :src="normalizeImage(item.detailImg)"
            :alt="item.c2cItemsName"
            width="480"
            height="480"
            loading="lazy"
            decoding="async"
            referrerpolicy="no-referrer"
            class="catalog-card__image"
            @error="event => fallbackToImageProxy(event, item.detailImg)"
          />
          <div v-else class="catalog-card__fallback">
            <SvgIcon icon="mdi:image-off-outline" class="text-28px text-#94a3b8" />
          </div>
          <span class="catalog-card__count">{{ item.itemCount }} 件发布</span>
        </div>

        <div class="catalog-card__content">
          <p class="catalog-card__title">{{ item.c2cItemsName }}</p>
          <p class="catalog-card__meta">
            {{ resolveReferencePriceLabel(item.referencePriceLabel, item.referencePriceMin, item.referencePriceMax) }}
          </p>
          <span class="catalog-card__link">
            查看详情
            <icon-mdi-arrow-right class="text-15px" />
          </span>
        </div>
      </button>
    </div>

    <NEmpty v-else :description="emptyDescription" class="catalog-empty">
      <template #extra>
        <NButton secondary @click="resetFilters">清除筛选</NButton>
      </template>
    </NEmpty>

    <footer class="catalog-footer">
      <div class="catalog-footer__summary">
        <NSpin v-if="refreshing" size="small" />
        <span>共 {{ pagination.itemCount.toLocaleString() }} 个聚合商品</span>
      </div>
      <NPagination
        v-model:page="pagination.page"
        v-model:page-size="pagination.pageSize"
        :item-count="pagination.itemCount"
        :page-count="pagination.pageCount"
        :page-sizes="[12, 24, 36]"
        :page-slot="appStore.isMobile ? 5 : 9"
        :show-size-picker="!appStore.isMobile"
        @update:page="() => search()"
        @update:page-size="() => search(true)"
      />
    </footer>

    <NDrawer v-model:show="showMobileFilters" placement="bottom" height="78vh">
      <NDrawerContent title="筛选与排序" closable>
        <div class="mobile-filter-form">
          <div class="catalog-filter-field">
            <div class="catalog-filter-field__label">
              <span>发布时间</span>
              <NSwitch v-model:value="timeRangeEnable" size="small" />
            </div>
            <NDatePicker
              v-model:value="timeRange"
              type="datetimerange"
              clearable
              :disabled="!timeRangeEnable"
              class="catalog-filter-control"
            />
          </div>

          <div class="catalog-filter-field">
            <div class="catalog-filter-field__label">
              <span>参考价格</span>
              <NSwitch v-model:value="priceRangeEnable" size="small" />
            </div>
            <NInputGroup vertical>
              <NInputNumber v-model:value="priceRange[0]" :precision="2" :disabled="!priceRangeEnable">
                <template #suffix>元</template>
              </NInputNumber>
              <NInputNumber v-model:value="priceRange[1]" :precision="2" :disabled="!priceRangeEnable">
                <template #suffix>元</template>
              </NInputNumber>
            </NInputGroup>
          </div>

          <div class="catalog-filter-field">
            <div class="catalog-filter-field__label"><span>排序方式</span></div>
            <NRadioGroup v-model:value="sortOpt" name="catalogSortMobile" class="mobile-sort-group">
              <NRadioButton
                v-for="sortway in sortways"
                :key="sortway.value"
                :value="sortway.value"
                :label="sortway.label"
              />
            </NRadioGroup>
          </div>
        </div>

        <template #footer>
          <div class="mobile-filter-actions">
            <NButton secondary @click="resetFilters">重置</NButton>
            <NButton type="primary" @click="applyMobileFilters">应用筛选</NButton>
          </div>
        </template>
      </NDrawerContent>
    </NDrawer>
  </main>
</template>

<style scoped>
.catalog-page {
  display: flex;
  flex-direction: column;
  gap: var(--bsm-space-5);
  width: min(100%, 1680px);
  margin: 0 auto;
}

.catalog-hero {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 24px;
  padding: clamp(22px, 3vw, 36px);
  overflow: hidden;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  background:
    radial-gradient(circle at 88% 10%, rgba(251, 114, 153, 0.22), transparent 34%),
    linear-gradient(135deg, var(--bsm-surface) 0%, color-mix(in srgb, var(--bsm-primary) 8%, var(--bsm-surface)) 100%);
  box-shadow: var(--bsm-shadow-sm);
}

.catalog-hero__eyebrow {
  margin: 0 0 8px;
  color: var(--bsm-primary);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.16em;
}

.catalog-hero__title {
  margin: 0;
  color: var(--bsm-text);
  font-size: clamp(26px, 4vw, 40px);
  line-height: 1.15;
  letter-spacing: -0.035em;
}

.catalog-hero__description {
  max-width: 680px;
  margin: 12px 0 0;
  color: var(--bsm-text-muted);
  font-size: 14px;
  line-height: 1.7;
}

.catalog-hero__summary {
  display: flex;
  flex: 0 0 auto;
  flex-direction: column;
  gap: 4px;
  min-width: 170px;
  padding: 14px 18px;
  border: 1px solid color-mix(in srgb, var(--bsm-primary) 20%, transparent);
  border-radius: var(--bsm-radius-lg);
  background: color-mix(in srgb, var(--bsm-surface) 82%, transparent);
  backdrop-filter: blur(16px);
}

.catalog-hero__summary-label {
  color: var(--bsm-text-muted);
  font-size: 12px;
}

.catalog-hero__summary strong {
  color: var(--bsm-text);
  font-size: 18px;
}

.catalog-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-lg);
  background: var(--bsm-surface);
  box-shadow: var(--bsm-shadow-xs);
}

.catalog-toolbar__search {
  min-width: 220px;
  flex: 1;
}

.catalog-filter-panel {
  padding: 20px;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  background: var(--bsm-surface);
  box-shadow: var(--bsm-shadow-xs);
}

.catalog-filter-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}

.catalog-filter-panel__header h2 {
  margin: 0;
  color: var(--bsm-text);
  font-size: 16px;
}

.catalog-filter-panel__header p {
  margin: 4px 0 0;
  color: var(--bsm-text-muted);
  font-size: 12px;
}

.catalog-filter-grid {
  display: grid;
  grid-template-columns: minmax(280px, 1.25fr) minmax(260px, 1fr) minmax(320px, 1.25fr);
  gap: 16px;
}

.catalog-filter-field {
  min-width: 0;
}

.catalog-filter-field__label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 22px;
  margin-bottom: 8px;
  color: var(--bsm-text);
  font-size: 13px;
  font-weight: 650;
}

.catalog-filter-control {
  width: 100%;
}

.catalog-sort-group {
  display: flex;
  width: 100%;
}

.catalog-sort-group :deep(.n-radio-button) {
  flex: 1;
}

.catalog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: clamp(12px, 1.5vw, 20px);
  transition: opacity 0.2s ease;
}

.catalog-grid.is-refreshing {
  opacity: 0.72;
}

.catalog-card {
  position: relative;
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 14px;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  padding: 12px;
  text-align: left;
  color: inherit;
  background: var(--bsm-surface);
  box-shadow: var(--bsm-shadow-xs);
  cursor: pointer;
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease,
    border-color 0.2s ease;
}

.catalog-card:hover,
.catalog-card:focus-visible {
  transform: translateY(-3px);
  border-color: color-mix(in srgb, var(--bsm-primary) 48%, var(--bsm-border));
  box-shadow: var(--bsm-shadow-md);
  outline: none;
}

.catalog-card__image-shell {
  position: relative;
  overflow: hidden;
  border-radius: calc(var(--bsm-radius-xl) - 5px);
  aspect-ratio: 1 / 1;
  background:
    linear-gradient(135deg, rgba(251, 114, 153, 0.12), rgba(99, 102, 241, 0.12)),
    var(--bsm-surface-muted);
}

.catalog-card__image {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.35s ease;
}

.catalog-card:hover .catalog-card__image {
  transform: scale(1.035);
}

.catalog-card__fallback {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
}

.catalog-card__count {
  position: absolute;
  right: 8px;
  bottom: 8px;
  padding: 4px 8px;
  border-radius: 999px;
  color: #fff;
  background: rgba(15, 23, 42, 0.72);
  font-size: 11px;
  font-weight: 650;
  backdrop-filter: blur(8px);
}

.catalog-card__content {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
  padding: 0 2px 2px;
}

.catalog-card__title {
  display: -webkit-box;
  overflow: hidden;
  margin: 0;
  color: var(--bsm-text);
  font-size: 15px;
  font-weight: 750;
  line-height: 1.5;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.catalog-card__meta {
  overflow: hidden;
  margin: 0;
  color: var(--bsm-text-muted);
  font-size: 12px;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.catalog-card__link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  margin-top: 2px;
  color: var(--bsm-primary);
  font-size: 12px;
  font-weight: 650;
}

.catalog-card--skeleton {
  cursor: default;
}

.catalog-card__skeleton-image {
  aspect-ratio: 1 / 1;
  border-radius: calc(var(--bsm-radius-xl) - 5px);
}

.catalog-empty {
  padding: 72px 0;
  border: 1px dashed var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  background: var(--bsm-surface);
}

.catalog-footer {
  position: sticky;
  bottom: 10px;
  z-index: 4;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-lg);
  background: color-mix(in srgb, var(--bsm-surface) 92%, transparent);
  box-shadow: var(--bsm-shadow-sm);
  backdrop-filter: blur(18px);
}

.catalog-footer__summary {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--bsm-text-muted);
  font-size: 13px;
}

.mobile-filter-form {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.mobile-sort-group {
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
}

.mobile-filter-actions {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 10px;
  width: 100%;
}

@media (max-width: 1100px) {
  .catalog-filter-grid {
    grid-template-columns: 1fr 1fr;
  }

  .catalog-filter-field:last-child {
    grid-column: 1 / -1;
  }
}

@media (max-width: 640px) {
  .catalog-page {
    gap: 12px;
  }

  .catalog-hero {
    align-items: flex-start;
    flex-direction: column;
    padding: 20px;
  }

  .catalog-hero__description {
    margin-top: 8px;
    font-size: 13px;
  }

  .catalog-hero__summary {
    width: 100%;
    min-width: 0;
  }

  .catalog-toolbar {
    display: grid;
    grid-template-columns: 1fr auto auto;
  }

  .catalog-toolbar__search {
    grid-column: 1 / -1;
    width: 100%;
  }

  .catalog-toolbar__primary {
    min-width: 96px;
  }

  .catalog-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .catalog-card {
    gap: 10px;
    border-radius: var(--bsm-radius-lg);
    padding: 8px;
  }

  .catalog-card__title {
    font-size: 13px;
  }

  .catalog-card__link {
    display: none;
  }

  .catalog-footer {
    position: static;
    align-items: stretch;
    flex-direction: column;
    padding: 12px;
  }

  .catalog-footer :deep(.n-pagination) {
    justify-content: center;
  }
}

@media (max-width: 380px) {
  .catalog-grid {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .catalog-card,
  .catalog-card__image,
  .catalog-grid {
    transition: none;
  }

  .catalog-card:hover {
    transform: none;
  }
}
</style>
