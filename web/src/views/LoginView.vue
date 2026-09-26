<script setup lang="ts">
// 登录页：调用 /user/login，成功后写入 store 并跳转项目列表。
import { LogInOutline } from '@vicons/ionicons5'
import { NButton, NForm, NFormItem, NIcon, NInput, useMessage } from 'naive-ui'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login } from '@/api/auth'
import { syncLocaleFromUser } from '@/composables/locale'
import { ErrCode } from '@/types/api'
import { useUserStore } from '@/stores/user'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const message = useMessage()
const user = useUserStore()
const { t } = useI18n()
const form = ref({ name: 'admin', pwd: 'admin123' })
const loading = ref(false)

async function submit(): Promise<void> {
  loading.value = true
  try {
    const res = await login(form.value)
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('auth.loginFailed'))
      return
    }
    user.setUser({ ...res.data.user, token: res.data.token })
    // 登录后按用户表偏好切换界面语言（跨设备一致）。
    syncLocaleFromUser(res.data.user.lang)
    message.success(t('auth.loginDone'))
    void router.push('/projects')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <n-form class="login-card" @submit.prevent="submit">
      <div class="login-head">
        <span class="login-icon"><n-icon :component="LogInOutline" /></span>
        <div>
          <h1 class="login-title">{{ t('auth.loginTitle') }}</h1>
          <p class="login-sub">{{ t('auth.loginSub') }}</p>
        </div>
      </div>
      <n-form-item :label="t('auth.username')">
        <n-input v-model:value="form.name" :placeholder="t('auth.usernamePlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('auth.password')">
        <n-input v-model:value="form.pwd" type="password" show-password-on="click" :placeholder="t('auth.passwordPlaceholder')" @keyup.enter="submit" />
      </n-form-item>
      <n-button type="primary" block :loading="loading" @click="submit">{{ t('auth.login') }}</n-button>
      <p class="login-alt">{{ t('auth.noAccount') }}<router-link class="link" to="/register">{{ t('auth.goRegister') }}</router-link></p>
    </n-form>
  </div>
</template>

<style scoped>
.login-alt {
  margin: 12px 0 0;
  font-size: 13px;
  color: var(--app-muted);
  text-align: center;
}
.login {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: #f5f7fa;
}
.login-card {
  width: 360px;
  padding: 24px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
}
.login-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}
.login-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 8px;
  background: rgba(24, 160, 88, 0.12);
  color: var(--app-accent);
  font-size: 22px;
  flex: 0 0 auto;
}
.login-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}
.login-sub {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--app-muted);
}
</style>
