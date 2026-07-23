<script setup lang="ts">
import { nextTick, reactive, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import { PageTab } from '@sa/materials';
import { useAppStore } from '@/store/modules/app';
import { useThemeStore } from '@/store/modules/theme';
import { useRouteStore } from '@/store/modules/route';
import { useTabStore } from '@/store/modules/tab';
import ContextMenu from './context-menu.vue';

defineOptions({
  name: 'GlobalTab'
});

const route = useRoute();
const appStore = useAppStore();
const themeStore = useThemeStore();
const routeStore = useRouteStore();
const tabStore = useTabStore();

const tabRef = ref<HTMLElement>();

const TAB_DATA_ID = 'data-tab-id';

type TabNamedNodeMap = NamedNodeMap & {
  [TAB_DATA_ID]: Attr;
};

async function scrollToActiveTab() {
  await nextTick();
  if (!tabRef.value) return;

  const { children } = tabRef.value;

  for (let i = 0; i < children.length; i += 1) {
    const child = children[i];

    const { value: tabId } = (child.attributes as TabNamedNodeMap)[TAB_DATA_ID];

    if (tabId === tabStore.activeTabId) {
      child.scrollIntoView({
        behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
        block: 'nearest',
        inline: 'center'
      });
      break;
    }
  }
}

function getContextMenuDisabledKeys(tabId: string) {
  const disabledKeys: App.Global.DropdownKey[] = [];

  if (tabStore.isTabRetain(tabId)) {
    const homeDisable: App.Global.DropdownKey[] = ['closeCurrent', 'closeLeft'];
    disabledKeys.push(...homeDisable);
  }

  return disabledKeys;
}

async function handleCloseTab(tab: App.Global.Tab) {
  await tabStore.removeTab(tab.id);

  if (themeStore.resetCacheStrategy === 'close') {
    routeStore.resetRouteCache(tab.routeKey);
  }
}

async function refresh() {
  appStore.reloadPage(500);
}

interface DropdownConfig {
  visible: boolean;
  x: number;
  y: number;
  tabId: string;
}

const dropdown: DropdownConfig = reactive({
  visible: false,
  x: 0,
  y: 0,
  tabId: ''
});

function setDropdown(config: Partial<DropdownConfig>) {
  Object.assign(dropdown, config);
}

let isClickContextMenu = false;

function handleDropdownVisible(visible: boolean) {
  if (!isClickContextMenu) {
    setDropdown({ visible });
  }
}

async function handleContextMenu(e: MouseEvent, tabId: string) {
  e.preventDefault();

  const { clientX, clientY } = e;

  isClickContextMenu = true;

  const DURATION = dropdown.visible ? 150 : 0;

  setDropdown({ visible: false });

  setTimeout(() => {
    setDropdown({
      visible: true,
      x: clientX,
      y: clientY,
      tabId
    });
    isClickContextMenu = false;
  }, DURATION);
}

function init() {
  tabStore.initTabStore(route);
}

function removeFocus() {
  (document.activeElement as HTMLElement)?.blur();
}

// watch
watch(
  () => route.fullPath,
  () => {
    tabStore.addTab(route);
  }
);
watch(
  () => tabStore.activeTabId,
  () => {
    scrollToActiveTab();
  }
);

// init
init();
</script>

<template>
  <DarkModeContainer class="size-full flex-y-center px-16px shadow-tab">
    <div class="native-tab-scroll h-full flex-1-hidden overflow-x-auto" @click="removeFocus">
      <div
        ref="tabRef"
        class="h-full flex pr-18px"
        :class="[themeStore.tab.mode === 'chrome' ? 'items-end' : 'items-center gap-12px']"
      >
        <PageTab
          v-for="tab in tabStore.tabs"
          :key="tab.id"
          :[TAB_DATA_ID]="tab.id"
          :mode="themeStore.tab.mode"
          :dark-mode="themeStore.darkMode"
          :active="tab.id === tabStore.activeTabId"
          :active-color="themeStore.themeColor"
          :closable="!tabStore.isTabRetain(tab.id)"
          @click="tabStore.switchRouteByTab(tab)"
          @close="handleCloseTab(tab)"
          @contextmenu="handleContextMenu($event, tab.id)"
        >
          <template #prefix>
            <SvgIcon :icon="tab.icon" :local-icon="tab.localIcon" class="inline-block align-text-bottom text-16px" />
          </template>
          <div class="max-w-240px ellipsis-text">{{ tab.label }}</div>
        </PageTab>
      </div>
    </div>
    <!-- <ReloadButton :loading="!appStore.reloadFlag" @click="refresh" /> -->
    <!-- <FullScreen :full="appStore.fullContent" @click="appStore.toggleFullContent" /> -->
  </DarkModeContainer>
  <ContextMenu
    :visible="dropdown.visible"
    :tab-id="dropdown.tabId"
    :disabled-keys="getContextMenuDisabledKeys(dropdown.tabId)"
    :x="dropdown.x"
    :y="dropdown.y"
    @update:visible="handleDropdownVisible"
  />
</template>

<style scoped>
.native-tab-scroll {
  scrollbar-width: none;
  overscroll-behavior-inline: contain;
}

.native-tab-scroll::-webkit-scrollbar {
  display: none;
}
</style>
