<script lang="ts" setup>
import { defineAsyncComponent } from 'vue';
import { useBoolean } from '@sa/hooks';
import { $t } from '@/locales';

defineOptions({ name: 'GlobalSearch' });

const { bool: show, toggle } = useBoolean();
const SearchModal = defineAsyncComponent(() => import('./components/search-modal.vue'));
</script>

<template>
  <ButtonIcon :tooltip-content="$t('common.search')" @click="toggle">
    <icon-uil-search />
  </ButtonIcon>
  <SearchModal v-if="show" v-model:show="show" />
</template>

<style lang="scss" scoped></style>
