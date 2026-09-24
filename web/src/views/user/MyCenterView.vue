<script setup lang="ts">
// 个人中心页：内容与顶栏头像浮层共用 UserAccountPanel，避免两处维护。
import UserAccountPanel from '@/components/UserAccountPanel.vue';
import { NCard } from 'naive-ui';
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
      <user-account-panel />
    </n-card>
  </div>
</template>

<style scoped>
.card {
  width: 460px;
}
</style>
