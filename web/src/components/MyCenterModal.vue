<script setup lang="ts">
// 个人中心弹窗：账号资料 + 访问令牌 tab 分栏（顶栏头像菜单「个人中心」触发）。
import PasswordModal from '@/components/PasswordModal.vue'
import ProfileModal from '@/components/ProfileModal.vue'
import TokenPanel from '@/components/TokenPanel.vue'
import { useLogout } from '@/composables/logout'
import { useUserStore } from '@/stores/user'
import { NButton, NForm, NFormItem, NInput, NModal, NSpace, NTabPane, NTabs } from 'naive-ui'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const show = defineModel<boolean>('show', { required: true })
const user = useUserStore()
const logout = useLogout()
const { t } = useI18n()
const showProfile = ref(false)
const showPwd = ref(false)
</script>

<template>
  <n-modal v-model:show="show" preset="card" :title="t('user.center')" style="width: 760px; max-width: 94vw">
    <n-tabs type="line" animated>
      <n-tab-pane name="profile" :tab="t('user.accountInfo')">
        <n-form label-placement="left" label-width="64">
          <n-form-item :label="t('user.account')"><n-input :value="user.userInfo?.name" disabled /></n-form-item>
          <n-form-item :label="t('user.nickName')"><n-input :value="user.userInfo?.nick_name || '—'" disabled /></n-form-item>
          <n-form-item :label="t('auth.email')"><n-input :value="user.userInfo?.email || '—'" disabled /></n-form-item>
        </n-form>
        <n-space>
          <n-button type="primary" size="small" @click="showProfile = true">{{ t('user.profile') }}</n-button>
          <n-button size="small" @click="showPwd = true">{{ t('user.password') }}</n-button>
          <n-button size="small" tertiary @click="logout">{{ t('user.logout') }}</n-button>
        </n-space>
      </n-tab-pane>

      <n-tab-pane name="tokens" :tab="t('user.tokenTitle')">
        <token-panel />
      </n-tab-pane>
    </n-tabs>

    <profile-modal v-model:show="showProfile" />
    <password-modal v-model:show="showPwd" />
  </n-modal>
</template>
