<script setup lang="ts">
// 环境选择器：切换「当前环境」（store + localStorage 按项目记忆），驱动变量解析结果实时重算。
import { useVariableStore } from '@/stores/variable'
import { NSelect } from 'naive-ui'
import { computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ projectId: number }>()

const store = useVariableStore()
const { t } = useI18n()

onMounted(() => void store.loadEnvs(props.projectId))

const options = computed(() => store.envList(props.projectId).map((e) => ({ label: e.name, value: e.id })))
const value = computed({
  get: () => store.currentEnv[props.projectId] ?? 0,
  set: (v: number) => store.setCurrentEnv(props.projectId, v),
})

/** 选项渲染：颜色点 + 名称 + 默认标记。 */
function renderLabel(option: { value: number }): ReturnType<typeof h> {
  const env = store.envList(props.projectId).find((e) => e.id === option.value)
  return h('span', { class: 'ep-opt' }, [
    h('span', { class: 'ep-dot', style: { background: env?.color || '#18a058' } }),
    h('span', null, env?.name ?? ''),
    env?.is_default === 1 ? h('span', { class: 'ep-tag' }, t('env.default')) : null,
  ])
}
</script>

<template>
  <span class="ep">
    <span class="ep-label">{{ t('env.label') }}</span>
    <n-select
      v-model:value="value"
      :options="options"
      :render-label="renderLabel"
      size="small"
      class="ep-select"
      :placeholder="t('env.none')"
      :consistent-menu-width="false"
    />
  </span>
</template>

<style scoped>
.ep {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ep-label {
  font-size: 12px;
  color: var(--app-muted);
}

.ep-select {
  width: 128px;
}

.ep-opt {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ep-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.ep-tag {
  padding: 0 4px;
  border-radius: 3px;
  background: #e8f6ee;
  color: var(--app-accent);
  font-size: 11px;
  line-height: 16px;
}
</style>
