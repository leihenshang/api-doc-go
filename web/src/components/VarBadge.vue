<script setup lang="ts">
// 接口的变量徽标：请求变量数（主色）+ 未定义数（告警色），悬停显示变量名清单。
// 数据来自 store.summaryFor（由容器调 store.summaryOf 批量拉取），与详情页共用同一套解析器。
import { useVariableStore } from '@/stores/variable'
import { NTooltip } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'

const props = defineProps<{ projectId: number; apiId: number }>()

const store = useVariableStore()
const { t } = useI18n()
const item = computed(() => store.summaryFor(props.projectId, props.apiId))
const req = computed(() => item.value?.request_count ?? 0)
const undef = computed(() => item.value?.undefined_count ?? 0)

/** 悬停清单：请求变量名 + 未定义的变量名（各最多 8 个）。 */
const tipText = computed(() => {
  const parts: string[] = []
  const defined = item.value?.defined ?? []
  const undefinedNames = item.value?.undefined ?? []
  const names = (list: string[]): string => list.slice(0, 8).join(t('common.listSep'))
  const more = (n: number): string => (n > 8 ? t('varBadge.more', { n }) : '')
  if (defined.length) parts.push(t('varBadge.request', { names: names(defined), more: more(defined.length) }))
  if (undefinedNames.length) {
    parts.push(t('varBadge.undefined', { names: names(undefinedNames), more: more(undefinedNames.length) }))
  }
  return parts.join(t('common.sentenceSep'))
})
</script>

<template>
  <!-- 根节点用普通元素（而非 tooltip）：父组件传入的 class（如 .gn-vars）才能合并到此处 -->
  <span v-if="req || undef" class="vb">
    <n-tooltip trigger="hover" :delay="150">
      <template #trigger>
        <span class="vb-nums">
          <span v-if="req" class="vb-req">{{ req }}</span>
          <span v-if="undef" class="vb-warn">{{ undef }}</span>
        </span>
      </template>
      {{ tipText }}
    </n-tooltip>
  </span>
</template>

<style scoped>
.vb,
.vb-nums {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

/* 请求变量数：主色实心小圆点样式 */
.vb-req,
.vb-warn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border: 1px solid;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
}

.vb-req {
  color: var(--app-accent);
  border-color: var(--app-accent);
  background: rgba(24, 160, 88, 0.1);
}

.vb-warn {
  color: #b7791f;
  border-color: #f0b429;
  background: #fff7e6;
}
</style>
