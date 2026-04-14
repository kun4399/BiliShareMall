import { onMounted, onUnmounted, ref } from 'vue';
import { GetLoginKeyAndUrl, GetSharedLoginSession, VerifyLogin } from '@/gateway';
import { resolveAppRuntime } from '@/gateway/runtime';
import { useAuthStore } from '@/store/modules/auth';
import { waitForSharedLoginSession } from './shared-session';

export function useBiliQrLogin() {
  const authStore = useAuthStore();
  const runtime = resolveAppRuntime();
  const loginUrl = ref('');
  const loading = ref(true);
  const statusText = ref('加载二维码中');

  let loginKey = '';
  let checkInterval: ReturnType<typeof setInterval> | null = null;
  let polling = false;

  function clearCheckInterval() {
    if (checkInterval) {
      clearInterval(checkInterval);
      checkInterval = null;
    }
  }

  async function initQrUrl() {
    loading.value = true;
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
      if (polling) {
        return;
      }
      polling = true;
      try {
        const ret = await VerifyLogin(loginKey);

        switch (ret.status) {
          case 'confirmed':
            if (ret.cookies !== '') {
              try {
                if (runtime === 'web') {
                  statusText.value = '登录成功，正在同步共享登录态';
                  const synced = await waitForSharedLoginSession(GetSharedLoginSession);
                  if (!synced) {
                    statusText.value = '登录已确认，但共享登录态尚未同步，正在重试';
                    break;
                  }
                } else {
                  statusText.value = '登录成功，正在进入应用';
                }
                await authStore.setCookies(ret.cookies);
                clearCheckInterval();
              } catch {
                statusText.value = runtime === 'web' ? '登录已确认，但共享登录态同步失败，请稍后重试' : '登录状态获取失败';
              }
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
            await initQrUrl();
            break;
          case 'error':
            statusText.value = ret.message || '登录状态获取失败';
            break;
          default:
            break;
        }
      } catch {
        statusText.value = '登录状态获取失败';
      } finally {
        polling = false;
      }
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
