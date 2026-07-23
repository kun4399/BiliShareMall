<script setup lang="ts">
import { computed, defineAsyncComponent, ref } from 'vue';
import type { VNode } from 'vue';
import { useAuthStore } from '@/store/modules/auth';
import { ClearAllLoginAccounts } from '@/gateway';
import { useRouterPush } from '@/hooks/common/router';
import { useSvgIcon } from '@/hooks/common/icon';
import { $t } from '@/locales';

defineOptions({
  name: 'UserAvatar'
});

type DropdownKey = 'accounts' | 'logout';

type DropdownOption = {
  key: DropdownKey;
  label: string;
  icon?: () => VNode;
};

const authStore = useAuthStore();
const { toLogin } = useRouterPush();
const { SvgIconVNode } = useSvgIcon();
const AccountManagerModal = defineAsyncComponent(() => import('./account-manager-modal.vue'));

const showAccountsModal = ref(false);

const options = computed<DropdownOption[]>(() => [
  {
    label: '账号管理',
    key: 'accounts',
    icon: SvgIconVNode({ icon: 'ph:users-three', fontSize: 18 })
  },
  {
    label: '全部退出',
    key: 'logout',
    icon: SvgIconVNode({ icon: 'ph:sign-out', fontSize: 18 })
  }
]);

function loginOrRegister() {
  toLogin();
}

function openAccountsModal() {
  showAccountsModal.value = true;
}

function clearAllWithConfirm() {
  window.$dialog?.warning({
    title: $t('common.tip'),
    content: '确认全部退出已添加账号吗？',
    positiveText: $t('common.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await ClearAllLoginAccounts();
        await authStore.resetStore();
        window.$message?.success('已全部退出');
      } catch (err: any) {
        window.$message?.error(err?.message || '全部退出失败');
      }
    }
  });
}

function handleDropdown(key: DropdownKey) {
  if (key === 'accounts') {
    openAccountsModal();
    return;
  }
  clearAllWithConfirm();
}
</script>

<template>
  <NButton v-if="!authStore.isLogin" quaternary @click="loginOrRegister">
    {{ $t('page.login.common.loginOrRegister') }}
  </NButton>

  <template v-else>
    <NDropdown placement="bottom-end" trigger="click" :options="options" @select="handleDropdown">
      <div>
        <ButtonIcon>
          <SvgIcon icon="ph:user-circle" class="text-icon-large" />
        </ButtonIcon>
      </div>
    </NDropdown>

    <AccountManagerModal v-if="showAccountsModal" v-model:show="showAccountsModal" />
  </template>
</template>
