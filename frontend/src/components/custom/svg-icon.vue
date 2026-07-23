<script setup lang="ts">
import { computed, useAttrs } from 'vue';
import { Icon } from '@iconify/vue';

defineOptions({ name: 'SvgIcon', inheritAttrs: false });

/**
 * Props
 *
 * - Support iconify and local svg icon
 * - If icon and localIcon are passed at the same time, localIcon will be rendered first
 */
interface Props {
  /** Iconify icon name */
  icon?: string;
  /** Local svg icon name */
  localIcon?: string;
}

const props = defineProps<Props>();

const attrs = useAttrs();
const localIconUrls = import.meta.glob('../../assets/svg-icon/*.svg', {
  eager: true,
  query: '?url',
  import: 'default'
}) as Record<string, string>;

const bindAttrs = computed<{ class: string; style: string }>(() => ({
  class: (attrs.class as string) || '',
  style: (attrs.style as string) || ''
}));

const localIconSrc = computed(
  () =>
    localIconUrls[`../../assets/svg-icon/${props.localIcon || 'no-icon'}.svg`] ||
    localIconUrls['../../assets/svg-icon/no-icon.svg']
);

/** If localIcon is passed, render localIcon first */
const renderLocalIcon = computed(() => props.localIcon || !props.icon);
</script>

<template>
  <template v-if="renderLocalIcon">
    <img aria-hidden="true" width="1em" height="1em" :src="localIconSrc" v-bind="bindAttrs" />
  </template>
  <template v-else>
    <Icon v-if="icon" :icon="icon" v-bind="bindAttrs" />
  </template>
</template>

<style scoped></style>
