<script setup lang="ts">
// 修改资料全局弹窗：打开时拉取最新资料，保存后同步用户 store。
import { me, updateProfile } from '@/api/auth'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import { NButton, NForm, NFormItem, NInput, NModal, useMessage } from 'naive-ui'
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const show = defineModel<boolean>('show', { required: true })
const store = useUserStore()
const message = useMessage()
const { t } = useI18n()
const saving = ref(false)
const profile = reactive({ nick_name: '', email: '' })

watch(show, (v) => {
  if (v) void load()
})

async function load(): Promise<void> {
  const res = await me()
  if (res.code === ErrCode.Success && res.data) {
    profile.nick_name = res.data.nick_name
    profile.email = res.data.email
  }
}

async function submit(): Promise<void> {
  saving.value = true
  try {
    const res = await updateProfile(store.userInfo?.id, { ...profile })
    if (res.code !== ErrCode.Success) {
      message.error(res.msg || t('common.saveFailed'))
      return
    }
    message.success(t('user.profileSaved'))
    if (res.data && store.userInfo) store.setUser({ ...res.data, token: store.token })
    show.value = false
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <n-modal v-model:show="show" preset="card" :title="t('user.profileTitle')" style="width: 420px">
    <n-form label-placement="left" label-width="64">
      <n-form-item :label="t('user.account')"><n-input :value="store.userInfo?.name" disabled /></n-form-item>
      <n-form-item :label="t('user.nickName')"><n-input v-model:value="profile.nick_name" :placeholder="t('user.nickNamePlaceholder')" /></n-form-item>
      <n-form-item :label="t('auth.email')"><n-input v-model:value="profile.email" :placeholder="t('auth.emailPlaceholder')" /></n-form-item>
    </n-form>
    <template #footer>
      <div class="foot">
        <n-button size="small" @click="show = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" size="small" :loading="saving" @click="submit">{{ t('common.save') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
