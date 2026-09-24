<script setup lang="ts">
// 左侧工作台树（Postman 集合面板样式）：分组节点（悬停行内新建子分组/重命名/删除）+ 其下的接口/文档叶子。
// 顶部搜索即时过滤叶子（接口匹配名称/URL，文档匹配标题），搜索态自动展开并隐藏无匹配的分组。
// 叶子点击经路由 query（apis → ?api=<id>，docs → ?doc=<id>）驱动右侧详情。
import { createGroup, deleteGroup, listApis, listDocs, updateGroup } from '@/api/content'
import MethodTag from '@/components/MethodTag.vue'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import { buildGroupTree, expandableIds, flattenTree, type FlatGroup } from '@/utils/group-tree'
import {
  AddOutline, ChevronForwardOutline, FolderOpenOutline, FolderOutline, PencilOutline, SearchOutline, TrashOutline,
} from '@vicons/ionicons5'
import { NButton, NIcon, NInput, useDialog, useMessage } from 'naive-ui'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/** 分组最大层级（顶级为第 1 级）。 */
const MAX_LEVEL = 7

/** 树叶：一个接口或文档。 */
interface TreeItem {
  id: number
  groupId: number
  title: string
  method?: string
  url?: string
}

const props = defineProps<{
  projectId: number
  type: number
  section: 'apis' | 'docs'
}>()

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()

const groups = computed(() => (props.section === 'apis' ? project.apiGroups : project.docGroups))
const roots = computed(() => buildGroupTree(groups.value))

const keyword = ref('')
const items = ref<TreeItem[]>([])

const expanded = ref<Set<number>>(new Set())
watch(roots, (r) => (expanded.value = new Set(expandableIds(r))), { immediate: true })

/** 当前选中叶子 id（来自路由 query）。 */
const selectedId = computed(() => {
  const key = props.section === 'apis' ? 'api' : 'doc'
  return Number(route.query[key]) || 0
})

const trashActive = computed(() => route.query.trash === '1')
const canCreate = computed(() => project.can(2))
const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))

/** 搜索过滤后的叶子（接口匹配名称/URL，文档匹配标题）。 */
const filteredItems = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return items.value
  return items.value.filter((it) => `${it.title} ${it.url ?? ''}`.toLowerCase().includes(kw))
})

/** 拉取全量叶子：分页上限 200，按页循环取全。 */
async function loadItems(): Promise<void> {
  const all: TreeItem[] = []
  const pageSize = 200
  for (let page = 1; page <= 20; page++) {
    if (props.section === 'apis') {
      const res = await listApis({ project_id: props.projectId, page_num: page, page_size: pageSize })
      if (res.code !== ErrCode.Success || !res.data) break
      res.data.items.forEach((a) => all.push({ id: a.id, groupId: a.group_id, title: a.api_name || '未命名接口', method: a.http_method_type, url: a.url }))
      if (all.length >= res.data.total) break
    } else {
      const res = await listDocs({ project_id: props.projectId, page_num: page, page_size: pageSize })
      if (res.code !== ErrCode.Success || !res.data) break
      res.data.items.forEach((d) => all.push({ id: d.id, groupId: d.group_id, title: d.title }))
      if (all.length >= res.data.total) break
    }
  }
  items.value = all
}

const searching = computed(() => keyword.value.trim().length > 0)

/** 按分组索引叶子（含 0=未分组）。 */
const itemsByGroup = computed(() => {
  const map = new Map<number, TreeItem[]>()
  for (const it of filteredItems.value) {
    const list = map.get(it.groupId) ?? []
    list.push(it)
    map.set(it.groupId, list)
  }
  return map
})

/** 可见分组行：搜索态显示全部层级且只留有匹配叶子的分组。 */
const visibleGroups = computed<FlatGroup[]>(() => {
  const flat = flattenTree(roots.value, (id) => searching.value || expanded.value.has(id))
  if (!searching.value) return flat
  const has = new Set<number>()
  const mark = (gid: number): void => {
    if (!gid || has.has(gid)) return
    has.add(gid)
    const parent = groups.value.find((g) => g.id === gid)?.p_id ?? 0
    if (parent) mark(parent)
  }
  for (const it of filteredItems.value) mark(it.groupId)
  return flat.filter((g) => has.has(g.id))
})

const ungroupedItems = computed(() => itemsByGroup.value.get(0) ?? [])

interface Row {
  kind: 'group' | 'item'
  level: number
  group?: FlatGroup
  item?: TreeItem
}

/** 渲染行序列：分组行 + 其叶子行（含未分组虚拟分组）。 */
const rows = computed<Row[]>(() => {
  const out: Row[] = []
  for (const g of visibleGroups.value) {
    out.push({ kind: 'group', level: g.level, group: g })
    for (const it of itemsByGroup.value.get(g.id) ?? []) out.push({ kind: 'item', level: g.level + 1, item: it })
  }
  if (!searching.value || ungroupedItems.value.length) {
    out.push({ kind: 'group', level: 1, group: { id: 0, title: '未分组', level: 1, hasChildren: false } })
    for (const it of ungroupedItems.value) out.push({ kind: 'item', level: 2, item: it })
  }
  return out
})

function goItem(it: TreeItem): void {
  const name = props.section === 'apis' ? 'projectApis' : 'projectDocs'
  const key = props.section === 'apis' ? 'api' : 'doc'
  void router.push({ name, params: { projectId: props.projectId }, query: { [key]: String(it.id) } })
}

function toggle(id: number): void {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

function expandOne(id: number): void {
  if (searching.value || expanded.value.has(id)) return
  const next = new Set(expanded.value)
  next.add(id)
  expanded.value = next
}

/** 选中项来自外部（如新建后跳转）时，逐级展开其所属分组，避免叶子不可见。 */
watch([selectedId, items], () => {
  const it = items.value.find((x) => x.id === selectedId.value)
  if (!it?.groupId) return
  const next = new Set(expanded.value)
  for (let gid: number | undefined = it.groupId; gid; gid = groups.value.find((g) => g.id === gid)?.p_id) {
    next.add(gid)
  }
  expanded.value = next
})

async function refresh(): Promise<void> {
  await project.loadGroups(props.projectId)
  await loadItems()
}

// ---- 分组管理（与原 GroupTree 一致） ----
const renamingId = ref(0)
const renameTitle = ref('')
const addingParent = ref<number | null>(null)
const addTitle = ref('')

function startAddRoot(): void {
  addingParent.value = 0
  addTitle.value = ''
}

function startAddChild(level: number, parentId: number): void {
  if (level >= MAX_LEVEL) {
    message.warning(`分组最多支持 ${MAX_LEVEL} 级`)
    return
  }
  addingParent.value = parentId
  addTitle.value = ''
}

async function submitAdd(): Promise<void> {
  if (addingParent.value === null) return
  const title = addTitle.value.trim()
  const parentId = addingParent.value === 0 ? 0 : addingParent.value
  addingParent.value = null
  addTitle.value = ''
  if (!title) return
  const res = await createGroup({ project_id: props.projectId, type: props.type, title, p_id: parentId })
  if (res.code !== ErrCode.Success) {
    message.error(res.msg || '创建失败')
    return
  }
  message.success('已创建分组')
  if (parentId) expandOne(parentId)
  await refresh()
}

function startRename(id: number, title: string): void {
  renamingId.value = id
  renameTitle.value = title
}

async function submitRename(): Promise<void> {
  if (!renamingId.value) return
  const id = renamingId.value
  const title = renameTitle.value.trim()
  renamingId.value = 0
  renameTitle.value = ''
  if (!title) return
  const res = await updateGroup({ id, title })
  if (res.code !== ErrCode.Success) {
    message.error(res.msg || '保存失败')
    return
  }
  message.success('已重命名')
  await refresh()
}

function hasChildren(id: number): boolean {
  return groups.value.some((g) => g.p_id === id)
}

function askDelete(node: FlatGroup): void {
  if (hasChildren(node.id)) {
    dialog.error({ title: '无法删除', content: '该分组下还有子分组，请先删除子分组。', positiveText: '知道了' })
    return
  }
  if ((itemsByGroup.value.get(node.id)?.length ?? 0) > 0) {
    dialog.error({ title: '无法删除', content: '该分组下还有接口/文档，请先移动或删除。', positiveText: '知道了' })
    return
  }
  dialog.warning({
    title: '删除分组',
    content: `确认删除分组「${node.title}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const res = await deleteGroup(node.id)
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || '删除失败')
        return
      }
      message.success('已删除')
      await refresh()
    },
  })
}

watch(() => props.projectId, () => void loadItems())
onMounted(() => {
  void loadItems()
})

defineExpose({ reload: refresh })
</script>

<template>
  <div class="gtree">
    <div class="gtree-hd">
      <span class="gtree-title">{{ section === 'apis' ? '接口集合' : '文档集合' }}</span>
      <span class="gtree-count meta">{{ items.length }}</span>
      <n-button v-if="canCreate" quaternary size="tiny" class="gtree-add" title="新建分组" @click="startAddRoot">
        <template #icon><n-icon :component="AddOutline" /></template>
      </n-button>
    </div>

    <div class="gtree-search">
      <n-input v-model:value="keyword" size="small" clearable :placeholder="section === 'apis' ? '搜索接口' : '搜索文档'">
        <template #prefix><n-icon :component="SearchOutline" /></template>
      </n-input>
    </div>

    <div class="gtree-list">
      <div v-if="addingParent === 0" class="gn gn--add" @click.stop>
        <span class="gn-caret" />
        <n-input v-model:value="addTitle" size="tiny" placeholder="分组名称" @keyup.enter="submitAdd" @blur="submitAdd" />
      </div>

      <template v-for="row in rows" :key="(row.kind === 'group' ? 'g' : 'i') + (row.group?.id ?? row.item?.id)">
        <!-- 分组行 -->
        <div
          v-if="row.kind === 'group' && row.group"
          class="gn gn--group"
          :style="{ paddingLeft: 6 + (row.level - 1) * 14 + 'px' }"
          @click="toggle(row.group.id)"
        >
          <button
            v-if="row.group.hasChildren"
            class="gn-caret"
            :aria-label="expanded.has(row.group.id) ? '折叠' : '展开'"
            @click.stop="toggle(row.group.id)"
          >
            <n-icon :component="ChevronForwardOutline" :class="{ 'is-open': searching || expanded.has(row.group.id) }" />
          </button>
          <span v-else class="gn-caret" />

          <n-icon
            class="gn-folder"
            :component="row.group.hasChildren && (searching || expanded.has(row.group.id)) ? FolderOpenOutline : FolderOutline"
          />

          <n-input
            v-if="renamingId === row.group.id"
            v-model:value="renameTitle"
            size="tiny"
            @click.stop
            @keyup.enter="submitRename"
            @blur="submitRename"
          />
          <span v-else class="gn-name ellipsis" :title="row.group.title">{{ row.group.title }}</span>

          <span v-if="renamingId !== row.group.id && row.group.id > 0" class="gn-actions">
            <button
              v-if="canCreate"
              title="新建子分组"
              :disabled="row.level >= MAX_LEVEL"
              @click.stop="startAddChild(row.level, row.group.id)"
            >
              <n-icon :component="AddOutline" />
            </button>
            <button v-if="canUpdate" title="重命名" @click.stop="startRename(row.group.id, row.group.title)">
              <n-icon :component="PencilOutline" />
            </button>
            <button v-if="canDelete" title="删除" @click.stop="askDelete(row.group)">
              <n-icon :component="TrashOutline" />
            </button>
          </span>
        </div>

        <!-- 分组行内新建子分组录入 -->
        <div
          v-if="row.kind === 'group' && row.group && addingParent === row.group.id"
          class="gn gn--add"
          :style="{ paddingLeft: 6 + row.level * 14 + 16 + 'px' }"
          @click.stop
        >
          <span class="gn-caret" />
          <n-input v-model:value="addTitle" size="tiny" placeholder="子分组名称" @keyup.enter="submitAdd" @blur="submitAdd" />
        </div>

        <!-- 叶子行：接口（方法 + 名称）/ 文档（标题） -->
        <div
          v-else-if="row.kind === 'item' && row.item"
          class="gn gn--leaf"
          :style="{ paddingLeft: 6 + (row.level - 1) * 14 + 16 + 'px' }"
          :class="{ 'gn--active': selectedId === row.item.id && !trashActive }"
          @click="goItem(row.item)"
        >
          <method-tag v-if="section === 'apis'" plain :method="row.item.method ?? ''" class="gn-method" />
          <span v-else class="gn-dot" />
          <span class="gn-name ellipsis" :title="row.item.title">{{ row.item.title }}</span>
        </div>
      </template>

      <div v-if="!rows.length" class="gtree-empty meta">{{ searching ? '无匹配结果' : '暂无内容' }}</div>
    </div>
  </div>
</template>

<style scoped>
/* 面板本体扁平化，容器（.split-tree）负责背景与右分隔线。 */
.gtree {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  height: 100%;
}

.gtree-hd {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 38px;
  padding: 0 8px 0 12px;
  border-bottom: 1px solid var(--app-border);
}

.gtree-title {
  font-size: 13px;
  font-weight: 600;
}

.gtree-count {
  flex: 1 1 auto;
}

.gtree-add {
  color: var(--app-muted);
}

.gtree-search {
  padding: 8px 10px;
}

.gtree-list {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 2px 4px 12px;
}

.gtree-empty {
  padding: 20px 10px;
  text-align: center;
}

.gn {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding-right: 6px;
  border-radius: 3px;
  cursor: pointer;
  font-size: 13px;
}

.gn:hover {
  background: #eceef1;
}

.gn--group {
  font-weight: 600;
  color: #3d444d;
}

.gn--leaf {
  color: var(--app-text);
}

.gn--active,
.gn--active:hover {
  color: var(--app-accent);
  background: #e3f3e9;
  font-weight: 600;
}

.gn-folder {
  flex: 0 0 auto;
  color: #9aa2ab;
  font-size: 14px;
}

.gn--leaf .gn-method {
  flex: 0 0 38px;
}

.gn-dot {
  flex: 0 0 auto;
  width: 4px;
  height: 4px;
  margin-right: 2px;
  border-radius: 50%;
  background: #b9bfc6;
}

.gn-caret {
  flex: 0 0 16px;
  width: 16px;
  height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  padding: 0;
  background: none;
  color: var(--app-muted);
  cursor: pointer;
}

.gn-caret .is-open {
  transform: rotate(90deg);
}

.gn-name {
  flex: 1 1 auto;
  min-width: 0;
}

.gn-actions {
  flex: 0 0 auto;
  display: none;
  align-items: center;
  gap: 2px;
}

.gn:hover .gn-actions {
  display: inline-flex;
}

.gn-actions button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border: 0;
  border-radius: 3px;
  background: none;
  color: var(--app-muted);
  cursor: pointer;
}

.gn-actions button:hover {
  color: var(--app-accent);
  background: #fff;
}

.gn-actions button[disabled] {
  cursor: not-allowed;
  opacity: 0.4;
}

.gn--add {
  cursor: default;
}

.gn--add .n-input {
  flex: 1 1 auto;
}
</style>
