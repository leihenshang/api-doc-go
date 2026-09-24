<script setup lang="ts">
// 工作台空态：大号浅色图标 + 引导文案 + 快捷操作提示（接口页与文档页共用）。
import { NIcon } from 'naive-ui'
import type { Component } from 'vue'

withDefaults(defineProps<{ icon: Component; title: string; tips?: { label: string; value: string }[] }>(), {
  tips: () => [],
})
</script>

<template>
  <div class="empty-hint">
    <n-icon :component="icon" :size="112" class="eh-icon" />
    <p class="eh-title">{{ title }}</p>
    <ul v-if="tips.length" class="eh-tips">
      <li v-for="t in tips" :key="t.label">
        <span>{{ t.label }}</span>
        <span class="eh-key">{{ t.value }}</span>
      </li>
    </ul>
    <slot />
  </div>
</template>

<style scoped>
.empty-hint {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 20px;
  text-align: center;
}

.eh-icon {
  color: #e2e6ea;
}

.eh-title {
  margin: 18px 0 0;
  font-size: 14px;
}

.eh-tips {
  list-style: none;
  margin: 6px 0 20px;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 320px;
}

.eh-tips li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  font-size: 12px;
  color: var(--app-muted);
}

.eh-key {
  color: #b3bac1;
}
</style>
