<script setup lang="ts">
import { computed } from 'vue';
import { getPaletteColorByNumber, mixColor } from '@sa/color';
import { $t } from '@/locales';
import { useAppStore } from '@/store/modules/app';
import { useThemeStore } from '@/store/modules/theme';
import BiliQrlogin from '@/views/_builtin/login/modules/bili-qrlogin.vue';

const appStore = useAppStore();
const themeStore = useThemeStore();

const bgThemeColor = computed(() =>
  themeStore.darkMode ? getPaletteColorByNumber(themeStore.themeColor, 600) : themeStore.themeColor
);

const bgColor = computed(() => {
  const COLOR_WHITE = '#ffffff';

  const ratio = themeStore.darkMode ? 0.5 : 0.2;

  return mixColor(COLOR_WHITE, themeStore.themeColor, ratio);
});
</script>

<template>
  <main class="login-page" :style="{ backgroundColor: bgColor }">
    <WaveBg :theme-color="bgThemeColor" />
    <NCard :bordered="false" class="login-card">
      <div class="login-card__inner">
        <header class="login-card__header">
          <div class="login-card__brand">
            <SystemLogo class="text-56px text-primary lt-sm:text-46px" />
            <div>
              <p>WELCOME BACK</p>
              <h1>{{ $t('system.title') }}</h1>
            </div>
          </div>
          <div class="login-card__tools">
            <ThemeSchemaSwitch
              :theme-schema="themeStore.themeScheme"
              :show-tooltip="false"
              class="text-20px lt-sm:text-18px"
              @switch="themeStore.toggleThemeScheme"
            />
            <LangSwitch
              :lang="appStore.locale"
              :lang-options="appStore.localeOptions"
              :show-tooltip="false"
              @change-lang="appStore.changeLocale"
            />
          </div>
        </header>
        <section class="login-card__content">
          <h2>扫码登录</h2>
          <p>使用哔哩哔哩 App 扫描二维码，登录态会安全保存在当前应用中。</p>
          <Transition :name="themeStore.page.animateMode" mode="out-in" appear>
            <BiliQrlogin />
          </Transition>
        </section>
      </div>
    </NCard>
  </main>
</template>

<style scoped>
.login-page {
  position: relative;
  display: grid;
  width: 100%;
  height: 100%;
  min-height: 620px;
  place-items: center;
  overflow: hidden;
  padding: 24px;
}

.login-card {
  position: relative;
  z-index: 4;
  width: min(520px, 100%);
  border: 1px solid var(--bsm-border);
  border-radius: 24px;
  background: color-mix(in srgb, var(--bsm-surface) 90%, transparent);
  box-shadow: 0 30px 90px rgba(15, 23, 42, 0.2);
  backdrop-filter: blur(24px);
}

.login-card__inner {
  padding: 8px;
}

.login-card__header,
.login-card__brand {
  display: flex;
  align-items: center;
}

.login-card__header {
  justify-content: space-between;
  gap: 18px;
}

.login-card__brand {
  gap: 12px;
}

.login-card__brand p {
  margin: 0 0 2px;
  color: var(--bsm-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.14em;
}

.login-card__brand h1 {
  margin: 0;
  color: var(--bsm-text);
  font-size: 23px;
}

.login-card__tools {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.login-card__content {
  margin-top: 24px;
  padding: 24px;
  border: 1px solid var(--bsm-border);
  border-radius: var(--bsm-radius-xl);
  text-align: center;
  background: var(--bsm-surface-muted);
}

.login-card__content h2 {
  margin: 0;
  color: var(--bsm-text);
  font-size: 20px;
}

.login-card__content > p {
  margin: 8px auto 2px;
  color: var(--bsm-text-muted);
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 520px) {
  .login-page {
    min-height: 540px;
    padding: 12px;
  }

  .login-card :deep(.n-card__content) {
    padding: 14px;
  }

  .login-card__inner {
    padding: 0;
  }

  .login-card__content {
    margin-top: 16px;
    padding: 18px 10px;
  }
}
</style>
