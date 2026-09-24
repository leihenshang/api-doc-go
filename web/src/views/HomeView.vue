<script setup lang="ts">
// 首页占位：验证前端到后端连通性（调用 /api/health）。
import { ref } from 'vue'
import { NButton, NText, useMessage } from 'naive-ui'
import { get } from '@/api/http'

const status = ref('未检测')
const message = useMessage()

async function checkHealth() {
  const res = await get<{ status: string }>('/health')
  status.value = JSON.stringify(res.data)
  message.success(res.code === 200 ? '后端连通正常' : '后端返回异常')
}
</script>

<template>
  <div class="home">
    <h2>API-DOC · 脚手架已就绪</h2>
    <n-text>后端健康检查：{{ status }}</n-text>
    <br />
    <n-button type="primary" @click="checkHealth">检测 /api/health</n-button>
  </div>
</template>

<style scoped>
.home {
  padding: 48px;
}
</style>
