<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue';
import dayjs from 'dayjs';
import type { VNode } from 'vue';
import type { auth } from '~/wailsjs/go/models';
import { useAuthStore } from '@/store/modules/auth';
import {
  ClearAllLoginAccounts,
  DeleteLoginAccount,
  GetLoginKeyAndUrl,
  ListLoginAccounts,
  VerifyLogin
} from '@/gateway';
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

const showAccountsModal = ref(false);
const accountsLoading = ref(false);
const accounts = ref<auth.LoginAccount[]>([]);
const qrLoading = ref(false);
const loginUrl = ref('');
const statusText = ref('');
const polling = ref(false);

let loginKey = '';
let checkInterval: ReturnType<typeof setInterval> | null = null;

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

function clearCheckInterval() {
  if (checkInterval) {
    clearInterval(checkInterval);
    checkInterval = null;
  }
  polling.value = false;
}

function closeModal() {
  showAccountsModal.value = false;
  loginUrl.value = '';
  statusText.value = '';
  qrLoading.value = false;
  clearCheckInterval();
}

async function loadAccounts() {
  accountsLoading.value = true;
  try {
    accounts.value = await ListLoginAccounts();
    window.dispatchEvent(new Event('bsm-login-accounts-updated'));
  } catch (err: any) {
    window.$message?.error(err?.message || '账号列表加载失败');
  } finally {
    accountsLoading.value = false;
  }
}

function formatUpdatedAt(timestamp: number) {
  if (!timestamp) {
    return '-';
  }
  return dayjs(timestamp).format('YYYY-MM-DD HH:mm:ss');
}

async function openAccountsModal() {
  showAccountsModal.value = true;
  await loadAccounts();
}

async function startAddAccount() {
  qrLoading.value = true;
  statusText.value = '加载二维码中';
  clearCheckInterval();
  try {
    const loginInfo = await GetLoginKeyAndUrl();
    loginKey = loginInfo.key;
    loginUrl.value = loginInfo.login_url;
    statusText.value = '请使用哔哩哔哩 App 扫码登录';
    startLoginCheck();
  } catch (err: any) {
    statusText.value = err?.message || '二维码加载失败';
  } finally {
    qrLoading.value = false;
  }
}

function startLoginCheck() {
  clearCheckInterval();

  checkInterval = setInterval(async () => {
    if (polling.value || !loginKey) {
      return;
    }
    polling.value = true;

    try {
      const ret = await VerifyLogin(loginKey);
      switch (ret.status) {
        case 'confirmed':
          if (ret.cookies) {
            statusText.value = '账号添加成功';
            clearCheckInterval();
            await loadAccounts();
            window.$message?.success('账号添加成功');
          }
          break;
        case 'scanned':
          statusText.value = ret.message || '已扫码，请在手机上确认';
          break;
        case 'pending':
          statusText.value = ret.message || '等待扫码中';
          break;
        case 'expired':
          statusText.value = ret.message || '二维码已过期，请重新获取';
          clearCheckInterval();
          break;
        default:
          statusText.value = ret.message || '登录状态获取失败';
          break;
      }
    } catch (err: any) {
      statusText.value = err?.message || '登录状态获取失败';
    } finally {
      polling.value = false;
    }
  }, 3000);
}

async function handleDeleteAccount(account: auth.LoginAccount) {
  try {
    await DeleteLoginAccount(account.id);
    window.$message?.success('账号已退出');
    await loadAccounts();
  } catch (err: any) {
    window.$message?.error(err?.message || '账号退出失败');
  }
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
    void openAccountsModal();
    return;
  }
  clearAllWithConfirm();
}

onUnmounted(() => {
  clearCheckInterval();
});
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

    <NModal v-model:show="showAccountsModal" preset="card" title="账号管理" class="w-760px max-w-[95vw]" @after-leave="closeModal">
      <NSpace vertical size="small">
        <NSpace justify="space-between" align="center">
          <NText depth="3">可添加多个 Bilibili 账号并独立退出</NText>
          <NSpace size="small">
            <NButton size="small" :loading="qrLoading" @click="startAddAccount">添加账号</NButton>
            <NButton size="small" tertiary type="error" @click="clearAllWithConfirm">全部退出</NButton>
          </NSpace>
        </NSpace>

        <NCard v-if="loginUrl" size="small" class="account-qr-card">
          <NSpace align="center" size="small">
            <NQrCode :value="loginUrl" :size="128" :padding="0" />
            <NText depth="3">{{ statusText || '请扫码登录' }}</NText>
          </NSpace>
        </NCard>

        <NSpin :show="accountsLoading">
          <NEmpty v-if="accounts.length === 0" description="暂无账号，点击“添加账号”开始" />
          <NList v-else bordered size="small" class="accounts-list">
            <NListItem v-for="account in accounts" :key="account.id">
              <NFlex align="center" justify="space-between" :wrap="false" class="account-row">
                <NFlex vertical :size="2" class="account-main">
                  <NText class="account-name">{{ account.accountName || account.uid }}</NText>
                  <NText depth="3" class="account-meta">UID: {{ account.uid }}</NText>
                  <NText depth="3" class="account-meta">更新于：{{ formatUpdatedAt(account.updatedAt) }}</NText>
                </NFlex>
                <NSpace align="center" size="small">
                  <NTag :type="account.loggedIn ? 'success' : 'warning'" round size="small">
                    {{ account.loggedIn ? '已登录' : '未登录' }}
                  </NTag>
                  <NButton size="tiny" tertiary type="error" @click="handleDeleteAccount(account)">退出</NButton>
                </NSpace>
              </NFlex>
            </NListItem>
          </NList>
        </NSpin>
      </NSpace>
    </NModal>
  </template>
</template>

<style scoped>
.account-qr-card :deep(.n-card__content) {
  padding-top: 10px;
  padding-bottom: 10px;
}

.accounts-list :deep(.n-list-item) {
  padding-top: 8px;
  padding-bottom: 8px;
}

.account-row {
  width: 100%;
  gap: 10px;
}

.account-main {
  min-width: 0;
}

.account-name {
  font-weight: 600;
  line-height: 1.2;
}

.account-meta {
  font-size: 12px;
}

@media (max-width: 640px) {
  .account-row {
    align-items: flex-start;
    flex-wrap: wrap;
  }
}
</style>
