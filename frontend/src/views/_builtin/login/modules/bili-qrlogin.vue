<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue';
import { NQrCode } from 'naive-ui';
import { GetLoginKeyAndUrl, VerifyLogin } from '~/wailsjs/go/app/App';
import { useAuthStore } from '@/store/modules/auth';
defineOptions({
  name: 'BiliQrlogin'
});
const authStore = useAuthStore();
const loginUrl = ref('');
const loading = ref(true);
const statusText = ref('加载二维码中');

let loginKey: string = '';
let checkInterval: NodeJS.Timeout | null = null;
async function initQrurl() {
  GetLoginKeyAndUrl().then(loginInfo => {
    loginUrl.value = loginInfo.login_url;
    loginKey = loginInfo.key;
    loading.value = false;
    statusText.value = '请使用哔哩哔哩 App 扫码登录';
    startLoginCheck();
  });
}

function startLoginCheck() {
  if (checkInterval) {
    clearInterval(checkInterval); // 清除之前的定时器
  }
  checkInterval = setInterval(async () => {
    VerifyLogin(loginKey).then(ret => {
      switch (ret.status) {
        case 'confirmed':
          if (ret.cookies !== '') {
            statusText.value = '登录成功，正在进入应用';
            authStore.setCookies(ret.cookies);
            clearInterval(checkInterval!);
          }
          break;
        case 'scanned':
          statusText.value = ret.message || '已扫码，请在手机上确认登录';
          break;
        case 'pending':
          statusText.value = ret.message || '等待扫码中';
          break;
        case 'expired':
          statusText.value = ret.message || '二维码已过期，正在刷新';
          clearInterval(checkInterval!);
          initQrurl();
          break;
        case 'error':
          statusText.value = ret.message || '登录状态获取失败';
          break;
        default:
          break;
      }
    });
  }, 3000);
}

onMounted(async () => {
  await initQrurl();
});

onUnmounted(() => {
  if (checkInterval) {
    clearInterval(checkInterval);
  }
});
</script>

<template>
  <NSpin :show="loading" size="large" class="spin-container">
    <NSpace vertical>
      <NQrCode v-if="loginUrl" :value="loginUrl" :size="200" :padding="0" />
      <NText depth="3">{{ statusText }}</NText>
      <template #description>{{ statusText }}</template>
    </NSpace>
  </NSpin>
</template>

<style scoped>
.spin-container {
  display: flex;
  justify-content: center;
  height: 300px; /* 设置合适的高度 */
}
</style>
