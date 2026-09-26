<script setup lang="ts">
// 个人中心：展示账号资料，修改资料/改密走全局弹窗（与顶栏头像菜单一致）。
import PasswordModal from '@/components/PasswordModal.vue'
import ProfileModal from '@/components/ProfileModal.vue'
import { useLogout } from '@/composables/logout'
import { useUserStore } from '@/stores/user'
import { NButton, NCard, NForm, NFormItem, NInput, NSpace } from 'naive-ui'
import { ref } from 'vue'

const user = useUserStore()
const logout = useLogout()
const showProfile = ref(false)
const showPwd = ref(false)
</script>

<template>
  <div class="page">
    <div class="page-hd">
      <div>
        <h1 class="page-title">个人中心</h1>
        <p class="page-sub">维护本人资料与登录密码</p>
      </div>
    </div>

    <n-card :bordered="true" class="card">
      <n-form label-placement="left" label-width="64">
        <n-form-item label="账号"><n-input :value="user.userInfo?.name" disabled /></n-form-item>
        <n-form-item label="昵称"><n-input :value="user.userInfo?.nick_name || '—'" disabled /></n-form-item>
        <n-form-item label="邮箱"><n-input :value="user.userInfo?.email || '—'" disabled /></n-form-item>
      </n-form>
      <n-space>
        <n-button type="primary" size="small" @click="showProfile = true">修改资料</n-button>
        <n-button size="small" @click="showPwd = true">修改密码</n-button>
        <n-button size="small" tertiary @click="logout">退出登录</n-button>
      </n-space>
    </n-card>

    <profile-modal v-model:show="showProfile" />
    <password-modal v-model:show="showPwd" />
  </div>
</template>

<style scoped>
.card {
  width: 460px;
}
</style>
