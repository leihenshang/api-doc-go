<script setup lang="ts">
// 账户面板：基本资料与修改密码。供个人中心页与顶栏头像浮层共用。
import { me, updateProfile, updatePwd } from '@/api/auth'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import { NButton, NForm, NFormItem, NInput, NSpin, NTabPane, NTabs, useMessage } from 'naive-ui'
import { onMounted, reactive, ref } from 'vue'

const store = useUserStore()
const message = useMessage()
const emit = defineEmits<{ (e: 'logout'): void }>()

const loading = ref(false)
const saving = ref(false)
const profile = reactive({ nick_name: '', email: '' })
const pwd = reactive({ old_pwd: '', pwd: '', re_pwd: '' })

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await me()
    if (res.code === ErrCode.Success && res.data) {
      profile.nick_name = res.data.nick_name
      profile.email = res.data.email
    }
  } finally {
    loading.value = false
  }
}

async function saveProfile(): Promise<void> {
  saving.value = true
  try {
    const res = await updateProfile(store.userInfo?.id ?? 0, { ...profile })
    message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '资料已保存' : res.msg)
    if (res.code === ErrCode.Success && res.data && store.userInfo) {
      store.setUser({ ...res.data, token: store.token })
    }
  } finally {
    saving.value = false
  }
}

async function savePwd(): Promise<void> {
  if (!pwd.old_pwd || !pwd.pwd) {
    message.warning('请填写原密码与新密码')
    return
  }
  const res = await updatePwd({ ...pwd })
  message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '密码已修改' : res.msg)
  if (res.code === ErrCode.Success) Object.assign(pwd, { old_pwd: '', pwd: '', re_pwd: '' })
}

onMounted(load)
</script>

<template>
  <n-spin :show="loading">
    <div class="account-inner">
      <n-tabs type="segment" size="small">
        <n-tab-pane name="profile" tab="基本资料">
          <n-form label-placement="left" label-width="76" class="form">
            <n-form-item label="账号"><n-input :value="store.userInfo?.name" disabled /></n-form-item>
            <n-form-item label="昵称"><n-input v-model:value="profile.nick_name" /></n-form-item>
            <n-form-item label="邮箱"><n-input v-model:value="profile.email" /></n-form-item>
          </n-form>
          <n-button block type="primary" size="small" :loading="saving" @click="saveProfile">保存资料</n-button>
        </n-tab-pane>

        <n-tab-pane name="password" tab="修改密码">
          <n-form label-placement="left" label-width="76" class="form">
            <n-form-item label="原密码">
              <n-input v-model:value="pwd.old_pwd" type="password" show-password-on="click" />
            </n-form-item>
            <n-form-item label="新密码">
              <n-input v-model:value="pwd.pwd" type="password" show-password-on="click" />
            </n-form-item>
            <n-form-item label="确认密码">
              <n-input v-model:value="pwd.re_pwd" type="password" show-password-on="click" />
            </n-form-item>
          </n-form>
          <n-button block type="primary" size="small" @click="savePwd">修改密码</n-button>
        </n-tab-pane>
      </n-tabs>
      <div class="account-foot">
        <n-button block secondary size="small" @click="emit('logout')">退出登录</n-button>
      </div>
    </div>
  </n-spin>
</template>

<style scoped>
.form {
  margin-bottom: 12px;
}

.account-foot {
  margin-top: 12px;
  border-top: 1px solid var(--app-border);
  padding-top: 12px;
}

.account-inner {
  padding: 4px 2px;
}
</style>
