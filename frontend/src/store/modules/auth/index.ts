import { useRoute } from 'vue-router';
import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { SetupStoreId } from '@/enum';
import { useRouterPush } from '@/hooks/common/router';
import { resolveAppRuntime } from '@/gateway/runtime';
import { ClearSharedLoginSession, GetSharedLoginSession } from '@/gateway';
import { localStg } from '@/utils/storage';
import { $t } from '@/locales';
import { useRouteStore } from '../route';
import { useTabStore } from '../tab';
import { clearAuthStorage, getToken } from './shared';

export const useAuthStore = defineStore(SetupStoreId.Auth, () => {
  const route = useRoute();
  const routeStore = useRouteStore();
  const tabStore = useTabStore();
  const runtime = resolveAppRuntime();
  const token = ref(getToken());
  const webLoggedIn = ref(false);
  const initialized = ref(false);
  let initPromise: Promise<boolean> | null = null;

  const isLogin = computed(() => {
    return runtime === 'web' ? webLoggedIn.value : Boolean(token.value);
  });

  const { toLogin, redirectFromLogin } = useRouterPush(false);

  async function initAuthState(force = false) {
    if (runtime !== 'web') {
      token.value = getToken();
      initialized.value = true;
      return Boolean(token.value);
    }

    if (!force && initialized.value) {
      return webLoggedIn.value;
    }

    if (!force && initPromise) {
      return initPromise;
    }

    initPromise = GetSharedLoginSession()
      .then(session => {
        webLoggedIn.value = Boolean(session.loggedIn);
        initialized.value = true;
        return webLoggedIn.value;
      })
      .catch(() => {
        webLoggedIn.value = false;
        initialized.value = true;
        return false;
      })
      .finally(() => {
        initPromise = null;
      });

    return initPromise;
  }

  /** Reset auth store */
  async function resetStore() {
    const authStore = useAuthStore();

    if (runtime === 'web') {
      try {
        await ClearSharedLoginSession();
      } catch {
        // Ignore logout transport errors and still clear local state.
      }
      webLoggedIn.value = false;
    } else {
      clearAuthStorage();
      token.value = '';
    }

    authStore.$reset();
    token.value = runtime === 'web' ? '' : getToken();
    webLoggedIn.value = false;
    initialized.value = true;

    if (!route.meta.constant) {
      await toLogin();
    }

    tabStore.cacheTabs();
    routeStore.resetStore();
  }

  async function setCookies(cookies: string, redirect = true) {
    if (runtime === 'web') {
      webLoggedIn.value = true;
      initialized.value = true;
    } else {
      localStg.set('cookies', cookies);
      token.value = cookies;
    }
    await routeStore.initAuthRoute();
    await redirectFromLogin(redirect);
    if (routeStore.isInitAuthRoute) {
      window.$notification?.success({
        title: $t('page.login.common.loginSuccess'),
        content: $t('page.login.common.welcomeBack'),
        duration: 4500
      });
      tabStore.clearTabs();
    }
  }
  return {
    token,
    resetStore,
    setCookies,
    isLogin,
    initialized,
    initAuthState
  };
});
