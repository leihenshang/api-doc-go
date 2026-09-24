<script setup lang="ts">
// 注册页：用户名/密码/确认密码/邮箱/昵称；用户名与邮箱失焦即调 check-repeat 查重。
import { checkRepeat, register } from '@/api/auth'
import { ErrCode } from '@/types/api'
import { NButton, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const message = useMessage()

const form = reactive({ name: '', pwd: '', re_pwd: '', email: '', nick_name: '' })
const loading = ref(false)
const nameTaken = ref(false)
const emailTaken = ref(false)

/** 查重：字段有值时才校验，占用则标红提示。 */
async function check(kind: 'name' | 'email'): Promise<void> {
  const keyword = kind === 'name' ? form.name.trim() : form.email.trim()
  if (!keyword) return
  const res = await checkRepeat(keyword)
  const taken = res.code === ErrCode.Success && res.data?.repeated
  if (kind === 'name') nameTaken.value = Boolean(taken)
  else emailTaken.value = Boolean(taken)
  if (taken) message.warning(`「${keyword}」已被占用`)
}

async function submit(): Promise<void> {
  if (!form.name.trim() || !form.pwd) {
    message.warning('请填写用户名与密码')
    return
  }
  if (form.pwd !== form.re_pwd) {
    message.warning('两次输入的密码不一致')
    return
  }
  if (nameTaken.value || emailTaken.value) {
    message.warning('用户名或邮箱已被占用，请修改')
    return
  }
  loading.value = true
  try {
    const res = await register({
      name: form.name.trim(),
      pwd: form.pwd,
      re_pwd: form.re_pwd,
      email: form.email.trim() || undefined,
      nick_name: form.nick_name.trim() || undefined,
    })
    if (res.code !== ErrCode.Success) {
      message.error(res.msg || '注册失败')
      return
    }
    message.success('注册成功，请登录')
    void router.push({ name: 'login' })
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <n-form class="login-card" @submit.prevent="submit">
      <h1 class="login-title">注册账号</h1>
      <n-form-item label="用户名">
        <n-input
          v-model:value="form.name"
          placeholder="请输入用户名"
          :status="nameTaken ? 'error' : undefined"
          @blur="check('name')"
        />
      </n-form-item>
      <n-form-item label="密码">
        <n-input v-model:value="form.pwd" type="password" show-password-on="click" placeholder="请输入密码" />
      </n-form-item>
      <n-form-item label="确认密码">
        <n-input v-model:value="form.re_pwd" type="password" show-password-on="click" placeholder="再次输入密码" @keyup.enter="submit" />
      </n-form-item>
      <n-form-item label="邮箱">
        <n-input
          v-model:value="form.email"
          placeholder="可选"
          :status="emailTaken ? 'error' : undefined"
          @blur="check('email')"
        />
      </n-form-item>
      <n-form-item label="昵称">
        <n-input v-model:value="form.nick_name" placeholder="可选，缺省自动生成" />
      </n-form-item>
      <n-button type="primary" block :loading="loading" @click="submit">注册</n-button>
      <p class="login-alt">已有账号？<router-link class="link" to="/login">去登录</router-link></p>
    </n-form>
  </div>
</template>

<style scoped>
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

.login-title {
  margin: 0 0 16px;
  font-size: 18px;
  text-align: center;
}

.login-alt {
  margin: 12px 0 0;
  font-size: 13px;
  color: var(--app-muted);
  text-align: center;
}
</style>
