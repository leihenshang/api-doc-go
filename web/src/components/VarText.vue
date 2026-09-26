<script setup lang="ts">
// 变量就地渲染：把文本按 {{name}} 切片，命中显示强调色 + 悬停 tooltip（来源 / 取值），
// 空值中性提示，未定义给黄色告警。未提供 projectId/apiId 时退化为纯文本。
import { useVariableStore } from '@/stores/variable'
import { sourceLabel, splitVarSegments, type VarSegment } from '@/utils/vartext'
import { NTooltip } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'

const props = withDefaults(defineProps<{ text: string; projectId?: number; apiId?: number }>(), {
  projectId: 0,
  apiId: 0,
})

const store = useVariableStore()
const { t } = useI18n()
const map = computed(() => (props.projectId && props.apiId ? store.mapFor(props.projectId, props.apiId) : {}))
const segments = computed(() => splitVarSegments(props.text ?? '', map.value))

/** 悬停提示：变量名 · 来源 · 取值（敏感值只给掩码，空值提示「空」）。 */
function tip(s: VarSegment): string {
  if (s.state === 'undefined') return t('varText.undefined', { name: s.name })
  const kind = t(s.state === 'secret' ? 'varText.secret' : s.state === 'empty' ? 'varText.empty' : 'varText.value')
  return t('varText.tip', { name: s.name, source: sourceLabel(s.source), kind, value: s.value || t('varText.emptyValue') })
}
</script>

<template>
  <span class="vt">
    <template v-for="(s, i) in segments" :key="i">
      <span v-if="!s.name">{{ s.text }}</span>
      <n-tooltip v-else trigger="hover" :delay="150">
        <template #trigger>
          <span class="vt-var" :class="`vt-var--${s.state}`">{{ s.text }}</span>
        </template>
        {{ tip(s) }}
      </n-tooltip>
    </template>
  </span>
</template>

<style scoped>
.vt {
  word-break: break-all;
}

.vt-var {
  padding: 0 1px;
  border-bottom: 1px dashed currentColor;
  font-weight: 600;
  cursor: help;
}

/* 有值：强调色 */
.vt-var--value {
  color: var(--app-accent);
}

/* 敏感值：紫色（值本身是掩码） */
.vt-var--secret {
  color: #7a5af8;
}

/* 空值（已定义但值为空）：中性提示，不告警 */
.vt-var--empty {
  color: var(--app-muted);
  border-bottom-style: dotted;
}

/* 未定义：黄色告警底 */
.vt-var--undefined {
  color: #b7791f;
  background: #fff7e6;
  border-bottom-style: dotted;
  border-bottom-color: #f0b429;
  border-radius: 2px;
}
</style>
