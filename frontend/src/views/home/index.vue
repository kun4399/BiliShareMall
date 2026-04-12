<script setup lang="ts">
import { Search } from '@vicons/ionicons5';
import { useClipboard } from '@vueuse/core';
import { NButton, useMessage } from 'naive-ui';
import { h, onMounted, ref } from 'vue';
import { ListC2CItem } from '~/wailsjs/go/app/App';
import type { app } from '~/wailsjs/go/models';
import { getToken } from '@/store/modules/auth/shared';
const loading = ref(false);
const message = useMessage();
const searchText = ref('');

interface SortWay {
  value: number;
  /** The token */
  label: string;
}

const { copy, isSupported } = useClipboard();

async function handleCopy(item: app.C2CItemVO) {
  const copy_str = `https://mall.bilibili.com/neul-next/index.html?page=magic-market_detail&noTitleBar=1&itemsId=${item.c2cItemsId}`;
  if (!isSupported) {
    message.error(`复制失败，请自行复制链接：${copy_str}`);
    return;
  }
  await copy(copy_str);
  message.success('复制成功！');
}

const sortways = ref<SortWay[]>([
  { value: 1, label: '时间降序' },
  { value: 2, label: '价格升序' },
  { value: 3, label: '价格降序' }
]);

const columns = [
  {
    title: 'ID',
    key: 'c2cItemsId',
    width: 140
  },
  {
    title: '名称',
    key: 'c2cItemsName',
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '总数量',
    key: 'totalItemsCount',
    width: 100
  },
  {
    title: '价格',
    key: 'price',
    width: 100
  },
  {
    title: '链接',
    key: 'c2cItemsId',
    render(row: app.C2CItemVO) {
      return h(
        NButton,
        {
          size: 'small',
          onClick: () => handleCopy(row)
        },
        { default: () => '复制' }
      );
    },
    width: 80
  }
];
const timeRange = ref<[number, number]>([1183135260000, Date.now()]);
const timeRangeEnable = ref(false);
const priceRangeEnable = ref(false);
const used = ref(false);

const priceRange = ref([0, 9999]);
const sortOpt = ref(1);
const pagination = ref({
  page: 1,
  pageCount: 1,
  pageSize: 10
});
// 数据初始化
const data = ref<app.C2CItemVO[]>([]);

function search(firstPage: boolean = false) {
  loading.value = true;
  ListC2CItem(
    firstPage ? 1 : pagination.value.page,
    pagination.value.pageSize,
    searchText.value,
    sortOpt.value,
    timeRangeEnable.value ? timeRange.value[0] : -1,
    timeRangeEnable.value ? timeRange.value[1] : -1,
    priceRangeEnable.value ? priceRange.value[0] : -1,
    priceRangeEnable.value ? priceRange.value[1] : -1,
    used.value,
    getToken()
  )
    .then(result => {
      pagination.value.page = firstPage ? 1 : pagination.value.page;
      data.value = result.items;
      pagination.value.pageCount = result.totalPages;
      loading.value = false;
    })
    .catch(_err => {
      loading.value = false;
      message.error('请求失败');
    });
}
onMounted(() => {
  search();
});
</script>

<template>
  <NFlex>
    <NCard class="card-wrapper" title="数据库">
      <template #header-extra>
        <NSpace size="large">
          <NInput v-model:value="searchText" clearable :placeholder="$t('common.keywordSearch')">
            <template #prefix>
              <icon-uil-search class="text-15px text-#c2c2c2" />
            </template>
          </NInput>
          <NButton @click="() => search(true)">
            <template #icon>
              <Search></Search>
            </template>
            搜索
          </NButton>
        </NSpace>
      </template>
      <NCollapse default-expanded-names="3">
        <NCollapseItem title="爬取时间">
          <NDatePicker v-model:value="timeRange" type="datetimerange" clearable />
          <template #header-extra>
            <NSpace>
              <NSwitch v-model:value="timeRangeEnable" />
            </NSpace>
          </template>
        </NCollapseItem>
        <NCollapseItem title="价格">
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
        <NCollapseItem title="排序" name="3">
          <NFlex>
            <NRadioGroup v-model:value="sortOpt" name="productType">
              <NRadioButton
                v-for="product in sortways"
                :key="product.value"
                :value="product.value"
                :label="product.label"
              />
            </NRadioGroup>
          </NFlex>
        </NCollapseItem>
        <NCollapseItem title="过滤下架商品">
          <NAlert title="注意" type="info">
            1. 开启这个开关后，每一次搜索都会检验商品在当前时间是否可以购买。
            <br />
            2. 这个操作会减慢速度，因为会挨个检查是否可以购买
            <br />
            3. 检测到过期的商品都会删除
            <br />
            4. 为了防止检测过于频繁，5分钟内商品不会重复检测。但是这样会导致极少数情况下商品下架的情况
          </NAlert>
          <template #header-extra>
            <NSpace>
              <NSwitch v-model:value="used"></NSwitch>
            </NSpace>
          </template>
        </NCollapseItem>
      </NCollapse>
    </NCard>
    <NDataTable
      remote
      :data="data"
      :columns="columns"
      :loading="loading"
      :pagination="pagination"
      @update:page="
        page => {
          pagination.page = page;
          search();
        }
      "
    />
  </NFlex>
</template>

<style scoped></style>
