<script setup lang="ts">
// HTTP 方法标签：Postman 配色 + 描边徽标（浅底深字保证可读性，描边/底色取方法主色）。
import { computed } from 'vue'

/** 方法主色：GET 绿 / POST 橙 / PUT 蓝 / PATCH 紫 / DELETE 红，用于底色与描边。 */
const ACCENT: Record<string, string> = {
  GET: '#0cbb52',
  POST: '#ffb400',
  PUT: '#097bed',
  PATCH: '#8e44ad',
  DELETE: '#eb2013',
  HEAD: '#00a3a3',
  OPTIONS: '#7d8590',
}

/** 文字色：同色相压深，避免亮色前景在浅底上对比度不足。 */
const INK: Record<string, string> = {
  GET: '#0a7c39',
  POST: '#9a6200',
  PUT: '#0a5fbe',
  PATCH: '#71319a',
  DELETE: '#c41c10',
  HEAD: '#00756f',
  OPTIONS: '#565e66',
}

const props = defineProps<{ method: string }>()

const label = computed(() => (props.method || '').toUpperCase() || '—')
const accent = computed(() => ACCENT[label.value] ?? ACCENT.OPTIONS)
const ink = computed(() => INK[label.value] ?? INK.OPTIONS)
</script>

<template>
  <span class="mt" :style="{ color: ink, background: `${accent}1f`, borderColor: `${accent}80` }">{{ label }}</span>
</template>

<style scoped>
.mt {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  min-width: 52px;
  padding: 0 6px;
  border: 1px solid;
  border-radius: 3px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.5px;
  line-height: 17px;
}
</style>
