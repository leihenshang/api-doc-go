<script setup lang="ts">
// HTTP 方法标签：Postman 配色。默认渲染为浅底胶囊（列表/详情用），plain 为纯文字（树用）。
import { computed } from 'vue'

const props = withDefaults(defineProps<{ method: string; plain?: boolean }>(), { plain: false })

/** 方法主色：GET 绿 / POST 橙 / PUT 蓝 / PATCH 紫 / DELETE 红。 */
const COLORS: Record<string, string> = {
  GET: '#0cbb52',
  POST: '#ffb400',
  PUT: '#097bed',
  PATCH: '#8e44ad',
  DELETE: '#eb2013',
  HEAD: '#00a3a3',
  OPTIONS: '#7d8590',
}

const label = computed(() => (props.method || '').toUpperCase() || '—')
const color = computed(() => COLORS[label.value] ?? COLORS.OPTIONS)
/** 8 位十六进制末尾即 alpha，用主色 12% 作底。 */
const background = computed(() => (props.plain ? 'transparent' : `${color.value}1f`))
</script>

<template>
  <span class="mt" :class="{ 'mt--plain': plain }" :style="{ color, background }">{{ label }}</span>
</template>

<style scoped>
.mt {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  min-width: 54px;
  padding: 1px 6px;
  border-radius: 3px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.4px;
  line-height: 18px;
}

.mt--plain {
  min-width: 38px;
  padding: 0;
  justify-content: flex-start;
  font-size: 10px;
  line-height: 1;
}
</style>
