<script setup lang="ts">
// 顶栏全局搜索：跨项目检索 接口 / 项目文档（含正文）/ 项目，结果分组展示、点击直达。
// 可见性由后端按身份限定（管理员全量，其余仅参与的项目）。
import { searchAll, type SearchItem, type SearchResult } from '@/api/content'
import MethodTag from '@/components/MethodTag.vue'
import { ErrCode } from '@/types/api'
import { SearchOutline } from '@vicons/ionicons5'
import { NIcon, NInput, NSpin } from 'naive-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const keyword = ref('')
const loading = ref(false)
const open = ref(false)
const result = ref<SearchResult | null>(null)
/** 键盘上下键选中的下标（-1 表示未选）。 */
const activeIdx = ref(-1)

/** 扁平化结果顺序（接口 → 文档 → 项目），用于键盘导航与回车打开。 */
const flat = computed<SearchItem[]>(() => {
  const r = result.value
  if (!r) return []
  return [...r.apis, ...r.docs, ...r.projects]
})

/** 分组标题随语言切换（label 用 computed，切语言即时更新）。 */
const groups = computed(() => {
  const r = result.value
  if (!r) return []
  return [
    { key: 'apis', label: t('search.groupApis'), items: r.apis },
    { key: 'docs', label: t('search.groupDocs'), items: r.docs },
    { key: 'projects', label: t('search.groupProjects'), items: r.projects },
  ].filter((g) => g.items.length)
})

let timer: ReturnType<typeof setTimeout> | undefined
watch(keyword, (kw) => {
  clearTimeout(timer)
  if (!kw.trim()) {
    open.value = false
    result.value = null
    return
  }
  // 去抖：避免逐字请求
  timer = setTimeout(() => void run(kw.trim()), 300)
})

async function run(text: string): Promise<void> {
  loading.value = true
  open.value = true
  try {
    const res = await searchAll(text, 6)
    result.value = res.code === ErrCode.Success && res.data ? res.data : null
    activeIdx.value = -1
  } finally {
    loading.value = false
  }
}

/** 命中项跳转：接口/文档直达对应详情，项目进入其接口页。 */
function jump(item: SearchItem): void {
  open.value = false
  keyword.value = ''
  if (item.kind === 'api') {
    void router.push({ name: 'projectApis', params: { projectId: item.project_id }, query: { api: String(item.id) } })
    return
  }
  if (item.kind === 'doc') {
    void router.push({ name: 'projectDocs', params: { projectId: item.project_id }, query: { doc: String(item.id) } })
    return
  }
  void router.push({ name: 'projectApis', params: { projectId: item.project_id } })
}

function isActive(item: SearchItem): boolean {
  const current = activeIdx.value >= 0 ? flat.value[activeIdx.value] : undefined
  return Boolean(current && current.kind === item.kind && current.id === item.id)
}

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape') {
    open.value = false
    return
  }
  if (!flat.value.length) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    activeIdx.value = (activeIdx.value + 1) % flat.value.length
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    activeIdx.value = (activeIdx.value - 1 + flat.value.length) % flat.value.length
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    jump(flat.value[activeIdx.value >= 0 ? activeIdx.value : 0])
  }
}

/** 失焦延迟收起：让结果项的 mousedown 先触发。 */
function onBlur(): void {
  setTimeout(() => {
    open.value = false
  }, 150)
}

watch(() => route.fullPath, () => {
  open.value = false
})
</script>

<template>
  <div class="gs">
    <n-input
      v-model:value="keyword"
      size="small"
      clearable
      :placeholder="t('search.placeholder')"
      @keydown="onKeydown"
      @blur="onBlur"
      @focus="open = Boolean(keyword.trim())"
    >
      <template #prefix><n-icon :component="SearchOutline" /></template>
    </n-input>

    <div v-if="open" class="gs-panel">
      <n-spin :show="loading">
        <p v-if="!groups.length && !loading" class="gs-empty meta">{{ t('search.empty') }}</p>
        <div v-for="g in groups" :key="g.key" class="gs-group">
          <p class="gs-title">{{ g.label }}</p>
          <button
            v-for="it in g.items"
            :key="`${g.key}-${it.id}`"
            class="gs-item"
            :class="{ 'gs-item--active': isActive(it) }"
            @mousedown.prevent="jump(it)"
          >
            <span class="gs-main">
              <method-tag v-if="it.kind === 'api'" :method="it.method" class="gs-method" />
              <span class="gs-name ellipsis">{{ it.title }}</span>
            </span>
            <span class="gs-sub ellipsis">{{ it.subtitle }}</span>
            <span v-if="it.kind !== 'project'" class="gs-proj ellipsis">{{ it.project_name }}</span>
          </button>
        </div>
      </n-spin>
    </div>
  </div>
</template>

<style scoped>
.gs {
  position: relative;
  flex: 0 1 320px;
  width: 320px;
  margin: 0 16px;
}

.gs-panel {
  position: absolute;
  z-index: 20;
  top: 34px;
  left: 0;
  width: 440px;
  max-height: 60vh;
  overflow: auto;
  padding: 6px 0;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  background: #fff;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.1);
}

.gs-empty {
  margin: 0;
  padding: 14px 12px;
  text-align: center;
}

.gs-group + .gs-group {
  margin-top: 4px;
  border-top: 1px solid var(--app-border);
  padding-top: 4px;
}

.gs-title {
  margin: 0;
  padding: 4px 12px;
  color: var(--app-muted);
  font-size: 12px;
}

.gs-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 6px 12px;
  border: 0;
  background: none;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.gs-item:hover,
.gs-item--active {
  background: #f2f7f4;
}

.gs-main {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
  max-width: 44%;
}

.gs-name {
  font-weight: 600;
}

.gs-method {
  flex: 0 0 auto;
}

.gs-sub {
  flex: 1 1 auto;
  min-width: 0;
  color: var(--app-muted);
  font-size: 12px;
}

.gs-proj {
  flex: 0 0 auto;
  max-width: 30%;
  padding: 0 6px;
  border-radius: 3px;
  background: #f5f6f8;
  color: var(--app-muted);
  font-size: 12px;
}

/* ---- 移动端：搜索独占一行，结果面板占满屏宽 ---- */
@media (max-width: 768px) {
  .gs {
    order: 9;
    flex: 1 1 100%;
    width: auto;
    margin: 0;
  }

  .gs-panel {
    width: calc(100vw - 24px);
    max-height: 55vh;
  }

  .gs-main {
    max-width: 52%;
  }

  .gs-sub,
  .gs-proj {
    display: none;
  }
}
</style>
