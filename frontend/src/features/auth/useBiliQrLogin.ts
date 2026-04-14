import { onMounted, onUnmounted, ref } from 'vue';
import { GetLoginKeyAndUrl, VerifyLogin } from '@/gateway';
import { resolveAppRuntime } from '@/gateway/runtime';
import { useAuthStore } from '@/store/modules/auth';

export function useBiliQrLogin() {
  const authStore = useAuthStore();
  const runtime = resolveAppRuntime();
  const loginUrl = ref('');
  const loading = ref(true);
  const statusText = ref('加载二维码中');

  let loginKey = '';
  let checkInterval: ReturnType<typeof setInterval> | null = null;

  function clearCheckInterval() {
    if (checkInterval) {
      clearInterval(checkInterval);
      checkInterval = null;
    }
  }

  async function initQrUrl() {
    return GetLoginKeyAndUrl().then(loginInfo => {
      loginUrl.value = loginInfo.login_url;
      loginKey = loginInfo.key;
      loading.value = false;
      statusText.value = '请使用哔哩哔哩 App 扫码登录';
      startLoginCheck();
    });
  }

  function startLoginCheck() {
    clearCheckInterval();

    checkInterval = setInterval(async () => {
      VerifyLogin(loginKey).then(ret => {
        switch (ret.status) {
          case 'confirmed':
            if (ret.cookies !== '') {
              statusText.value = '登录成功，正在进入应用';
              authStore.setCookies(runtime === 'web' ? '' : ret.cookies);
              clearCheckInterval();
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
            clearCheckInterval();
            initQrUrl();
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
    await initQrUrl();
  });

  onUnmounted(() => {
    clearCheckInterval();
  });

  return {
    loginUrl,
    loading,
    statusText
  };
}
