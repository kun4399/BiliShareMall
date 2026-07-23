<script setup lang="ts">
import { NQrCode } from 'naive-ui';
import { useBiliQrLogin } from '@/features/auth/useBiliQrLogin';

defineOptions({
  name: 'BiliQrlogin'
});
const { loginUrl, loading, statusText } = useBiliQrLogin();
</script>

<template>
  <NSpin :show="loading" size="large" class="spin-container">
    <NSpace vertical align="center">
      <NQrCode v-if="loginUrl" :value="loginUrl" :size="200" :padding="0" />
      <NText depth="3" class="login-status">{{ statusText }}</NText>
      <template #description>{{ statusText }}</template>
    </NSpace>
  </NSpin>
</template>

<style scoped>
.spin-container {
  display: flex;
  justify-content: center;
  height: 270px;
  padding-top: 24px;
}

.login-status {
  display: block;
  max-width: 280px;
  text-align: center;
  line-height: 1.5;
}
</style>
