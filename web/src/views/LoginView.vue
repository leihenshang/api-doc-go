<script setup lang="ts">
// 登录页：调用 /user/login，成功后写入 store 并跳转项目列表。
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import { login } from '@/api/auth'
import { ErrCode } from '@/types/api'
import { useUserStore } from '@/stores/user'

const router = useRouter()
const message = useMessage()
const user = useUserStore()
const form = ref({ name: 'admin', pwd: 'admin123' })
const loading = ref(false)

async function submit(): Promise<void> {
  loading.value = true
  try {
    const res = await login(form.value)
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || '登录失败')
      return
    }
    user.setUser({ ...res.data.user, token: res.data.token })
    message.success('登录成功')
    void router.push('/projects')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <n-form class="login-card" @submit.prevent="submit">
      <n-form-item label="用户名">
        <n-input v-model:value="form.name" placeholder="请输入用户名" />
      </n-form-item>
      <n-form-item label="密码">
        <n-input v-model:value="form.pwd" type="password" show-password-on="click" placeholder="请输入密码" @keyup.enter="submit" />
      </n-form-item>
      <n-button type="primary" block :loading="loading" @click="submit">登录</n-button>
      <p class="login-alt">还没有账号？<router-link class="link" to="/register">去注册</router-link></p>
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
</style>
