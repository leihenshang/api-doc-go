<script setup lang="ts">
// 仓库样式标识：书形外框 + 装订脊线 + 内容线，随文字颜色（currentColor）。
</script>

<template>
  <svg
    class="repo-mark"
    viewBox="0 0 16 16"
    fill="none"
    stroke="currentColor"
    stroke-width="1.3"
    aria-hidden="true"
    focusable="false"
  >
    <rect x="2.35" y="1.35" width="11.3" height="12.3" rx="1.4" />
    <line x1="5.4" y1="1.35" x2="5.4" y2="13.65" />
    <line x1="7.7" y1="4.9" x2="11.3" y2="4.9" stroke-linecap="round" />
    <line x1="7.7" y1="7.4" x2="11.3" y2="7.4" stroke-linecap="round" />
  </svg>
</template>

<style scoped>
.repo-mark {
  display: block;
  width: 100%;
  height: 100%;
}
</style>
