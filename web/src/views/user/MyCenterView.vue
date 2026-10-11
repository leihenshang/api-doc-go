<script setup lang="ts">
// 个人中心（直链兜底页）：账号资料；修改资料/改密/访问令牌均走弹窗（顶栏头像菜单触发）。
import PasswordModal from '@/components/PasswordModal.vue'
import ProfileModal from '@/components/ProfileModal.vue'
import { useLogout } from '@/composables/logout'
import { useUserStore } from '@/stores/user'
import { NButton, NCard, NForm, NFormItem, NInput, NSpace } from 'naive-ui'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const user = useUserStore()
const logout = useLogout()
const { t } = useI18n()
const showProfile = ref(false)
const showPwd = ref(false)
</script>

<template>
  <div class="page">
    <div class="page-hd">
      <div>
        <h1 class="page-title">{{ t('user.center') }}</h1>
        <p class="page-sub">{{ t('user.centerSub') }}</p>
      </div>
    </div>

    <div class="center-wrap">
      <n-card :bordered="true">
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
      </n-card>
    </div>

    <profile-modal v-model:show="showProfile" />
    <password-modal v-model:show="showPwd" />
  </div>
</template>

<style scoped>
.center-wrap {
  max-width: 720px;
  margin: 0 auto;
}
</style>
