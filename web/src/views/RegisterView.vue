<script setup lang="ts">
// 注册页：用户名/密码/确认密码/邮箱/昵称；用户名与邮箱失焦即调 check-repeat 查重。
import { PersonAddOutline } from '@vicons/ionicons5'
import { checkRepeat, register } from '@/api/auth'
import { ErrCode } from '@/types/api'
import { NButton, NForm, NFormItem, NIcon, NInput, useMessage } from 'naive-ui'
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const router = useRouter()
const message = useMessage()
const { t } = useI18n()

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
  if (taken) message.warning(t('auth.taken', { keyword }))
}

async function submit(): Promise<void> {
  if (!form.name.trim() || !form.pwd) {
    message.warning(t('auth.required'))
    return
  }
  if (form.pwd !== form.re_pwd) {
    message.warning(t('auth.pwdMismatch'))
    return
  }
  if (nameTaken.value || emailTaken.value) {
    message.warning(t('auth.takenHint'))
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
      message.error(res.msg || t('auth.registerFailed'))
      return
    }
    message.success(t('auth.registerDone'))
    void router.push({ name: 'login' })
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <n-form class="login-card" @submit.prevent="submit">
      <div class="login-head">
        <span class="login-icon"><n-icon :component="PersonAddOutline" /></span>
        <div>
          <h1 class="login-title">{{ t('auth.registerTitle') }}</h1>
          <p class="login-sub">{{ t('auth.registerSub') }}</p>
        </div>
      </div>
      <n-form-item :label="t('auth.username')">
        <n-input
          v-model:value="form.name"
          :placeholder="t('auth.usernamePlaceholder')"
          :status="nameTaken ? 'error' : undefined"
          @blur="check('name')"
        />
      </n-form-item>
      <n-form-item :label="t('auth.password')">
        <n-input v-model:value="form.pwd" type="password" show-password-on="click" :placeholder="t('auth.passwordPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('auth.confirmPwd')">
        <n-input v-model:value="form.re_pwd" type="password" show-password-on="click" :placeholder="t('auth.confirmPwdPlaceholder')" @keyup.enter="submit" />
      </n-form-item>
      <n-form-item :label="t('auth.email')">
        <n-input
          v-model:value="form.email"
          :placeholder="t('auth.emailPlaceholder')"
          :status="emailTaken ? 'error' : undefined"
          @blur="check('email')"
        />
      </n-form-item>
      <n-form-item :label="t('auth.nickName')">
        <n-input v-model:value="form.nick_name" :placeholder="t('auth.nickNamePlaceholder')" />
      </n-form-item>
      <n-button type="primary" block :loading="loading" @click="submit">{{ t('auth.register') }}</n-button>
      <p class="login-alt">{{ t('auth.hasAccount') }}<router-link class="link" to="/login">{{ t('auth.goLogin') }}</router-link></p>
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

.login-alt {
  margin: 12px 0 0;
  font-size: 13px;
  color: var(--app-muted);
  text-align: center;
}
</style>
