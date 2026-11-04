<script setup lang="ts">
// 左侧工作台树（Postman 集合面板样式）：目录节点（悬停行内新建子目录/新建接口/重命名/删除）+ 其下的接口/文档叶子。
// 目录与其下叶子都可展开/收起（含仅含叶子的目录），顶部提供「全部展开/全部收起」。
// 顶部搜索即时过滤叶子（接口匹配名称/URL，文档匹配标题），搜索态自动展开并隐藏无匹配的目录。
// 叶子点击经路由 query（apis → ?api=<id>，docs → ?doc=<id>）驱动右侧详情；叶子删除收在名称后（悬停出现）。
import {
  createGroup, deleteApi, deleteDoc, deleteGroup, listApis, listDocs, sortNodes, updateGroup, type SortItem,
} from '@/api/content'
import MethodTag from '@/components/MethodTag.vue'
import { useProjectStore } from '@/stores/project'
import { useI18n } from 'vue-i18n'
import { ErrCode } from '@/types/api'
import { buildGroupTree, expandableIds, flattenTree, type FlatGroup } from '@/utils/group-tree'
import { DEFAULT_DOC_ICON, isImageIcon } from '@/utils/icon'
import {
  AddOutline, ChevronBackOutline, ChevronForwardOutline, ContractOutline, ExpandOutline, FolderOpenOutline,
  FolderOutline, PencilOutline, SearchOutline, TrashOutline,
} from '@vicons/ionicons5'
import { NButton, NDropdown, NIcon, NInput, useDialog, useMessage, type DropdownOption } from 'naive-ui'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/** 目录最大层级（顶级为第 1 级）。 */
const MAX_LEVEL = 7

/** 树叶：一个接口或文档。 */
interface TreeItem {
  id: number
  groupId: number
  title: string
  method?: string
  url?: string
  /** 文档图标（emoji 或图片 URL）。 */
  icon?: string
  /** all 模式必填：api 与 doc 是两张表，各自的主键会撞号，必须靠它区分。 */
  kind?: 'api' | 'doc'
}

/** 叶子的真实类型；旧模式（apis/docs）没有 kind 字段，按 section 兜底。 */
function itemKind(it: TreeItem): 'api' | 'doc' {
  return it.kind ?? (props.section === 'apis' ? 'api' : 'doc')
}

const props = defineProps<{
  projectId: number
  type: number
  /** all = 接口与文档混排在同一棵树里（对应客户端的单目录组织）。 */
  section: 'apis' | 'docs' | 'all'
  /** 是否显示「收起左栏」按钮（由容器提供收起后的展开入口）。 */
  collapsible?: boolean
}>()

const emit = defineEmits<{ (e: 'collapse'): void }>()

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const { t } = useI18n()
const project = useProjectStore()

const groups = computed(() => {
  if (props.section === 'all') return project.allGroups
  return props.section === 'apis' ? project.apiGroups : project.docGroups
})

/** 空标题分组属无效数据（如导入产生）：自身不渲染为「空文件夹」，其叶子并入「未分组」。 */
const blankGroupIds = computed(() => new Set(groups.value.filter((g) => !(g.title ?? '').trim()).map((g) => g.id)))
const roots = computed(() => buildGroupTree(groups.value.filter((g) => !blankGroupIds.value.has(g.id))))

const keyword = ref('')
const items = ref<TreeItem[]>([])

const expanded = ref<Set<number>>(new Set())

/** 可展开的目录 id：含子目录，或含接口/文档叶子（未分组用 0 表示）。 */
const expandableSet = computed(() => {
  const ids = new Set<number>(expandableIds(roots.value))
  for (const it of items.value) {
    const gid = groupKey(it)
    if (gid > 0) ids.add(gid)
  }
  if (items.value.some((it) => groupKey(it) === 0)) ids.add(0)
  return ids
})

/** 首次加载默认全展开；之后只补齐新出现的目录，避免刷新把用户手动收起的目录又展开。 */
let knownExpandable = new Set<number>()
watch(
  expandableSet,
  (ids) => {
    const next = new Set(expanded.value)
    for (const id of ids) if (!knownExpandable.has(id)) next.add(id)
    knownExpandable = new Set(ids)
    expanded.value = next
  },
  { immediate: true },
)

/** 是否已全部展开（决定「全部展开/全部收起」按钮的图标与语义）。 */
const allOpen = computed(() => {
  const ids = Array.from(expandableSet.value)
  return ids.length > 0 && ids.every((id) => expanded.value.has(id))
})

function toggleAll(): void {
  expanded.value = allOpen.value ? new Set() : new Set(expandableSet.value)
}

/** 当前选中叶子 id（来自路由 query）。 */
const selectedKind = computed<'api' | 'doc'>(() => {
  if (props.section === 'all') return Number(route.query.api) ? 'api' : 'doc'
  return props.section === 'apis' ? 'api' : 'doc'
})
const selectedId = computed(() => {
  if (props.section === 'all') return Number(route.query.api) || Number(route.query.doc) || 0
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
  // all 模式：两套叶子各拉各的。注意不能共用一个「拉满就 break」的循环 ——
  // 接口在第一页就拉满时会直接跳出，文档永远拿不到（曾经踩过）。
  if (props.section === 'apis' || props.section === 'all') {
    for (let page = 1; page <= 20; page++) {
      const res = await listApis({ project_id: props.projectId, page_num: page, page_size: pageSize })
      if (res.code !== ErrCode.Success || !res.data) break
      res.data.items.forEach((a) => all.push({
        id: a.id, groupId: a.group_id, title: a.api_name || t('tree.unnamedApi'),
        method: a.http_method_type, url: a.url, kind: 'api',
      }))
      if (all.length >= res.data.total) break
    }
  }
  if (props.section === 'docs' || props.section === 'all') {
    const base = all.length
    for (let page = 1; page <= 20; page++) {
      const res = await listDocs({ project_id: props.projectId, page_num: page, page_size: pageSize })
      if (res.code !== ErrCode.Success || !res.data) break
      res.data.items.forEach((d) => all.push({ id: d.id, groupId: d.group_id, title: d.title, icon: d.icon, kind: 'doc' }))
      if (all.length - base >= res.data.total) break
    }
  }
  items.value = all
}

const searching = computed(() => keyword.value.trim().length > 0)

/** 叶子的有效分组 key：空标题分组的叶子并入 0（未分组）。 */
function groupKey(it: TreeItem): number {
  return blankGroupIds.value.has(it.groupId) ? 0 : it.groupId
}

/** 按分组索引叶子（含 0=未分组）。 */
const itemsByGroup = computed(() => {
  const map = new Map<number, TreeItem[]>()
  for (const it of filteredItems.value) {
    const gid = groupKey(it)
    const list = map.get(gid) ?? []
    list.push(it)
    map.set(gid, list)
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
  for (const it of filteredItems.value) mark(groupKey(it))
  return flat.filter((g) => has.has(g.id))
})

const ungroupedItems = computed(() => itemsByGroup.value.get(0) ?? [])

/** 目录是否展开：搜索态强制展开，保证命中项可见。 */
function isOpen(g: FlatGroup): boolean {
  return searching.value || expanded.value.has(g.id)
}

/** 目录是否可展开/收起：含子目录或含叶子（仅含接口的目录同样可收起）。 */
function groupExpandable(g: FlatGroup): boolean {
  return g.hasChildren || (itemsByGroup.value.get(g.id)?.length ?? 0) > 0
}

interface Row {
  kind: 'group' | 'item'
  level: number
  group?: FlatGroup
  item?: TreeItem
}

/** 渲染行序列：目录行 + 其叶子行（含未分组虚拟目录）；目录收起时隐藏其叶子。 */
const rows = computed<Row[]>(() => {
  const out: Row[] = []
  for (const g of visibleGroups.value) {
    out.push({ kind: 'group', level: g.level, group: g })
    if (!isOpen(g)) continue
    for (const it of itemsByGroup.value.get(g.id) ?? []) out.push({ kind: 'item', level: g.level + 1, item: it })
  }
  if (ungroupedItems.value.length) {
    const virtualGroup: FlatGroup = { id: 0, title: t('tree.ungrouped'), level: 1, hasChildren: false }
    out.push({ kind: 'group', level: 1, group: virtualGroup })
    if (isOpen(virtualGroup)) {
      for (const it of ungroupedItems.value) out.push({ kind: 'item', level: 2, item: it })
    }
  }
  return out
})

function goItem(it: TreeItem): void {
  // 合并后只有「文档」一个工作台：接口用 ?api=，文档用 ?doc=，各自带 kind 避免同号串台。
  const name = props.section === 'apis' ? 'projectApis' : 'projectDocs'
  const key = itemKind(it)
  void router.push({ name, params: { projectId: props.projectId }, query: { [key]: String(it.id) } })
}

/** B19：按分组过滤右侧文档列表（group=0 = 未分组；不带 doc 参数避免跳到旧详情）。 */
function filterByGroup(id: number): void {
  void router.push({ name: 'projectDocs', params: { projectId: props.projectId }, query: { group: String(id) } })
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
  const it = items.value.find((x) => x.id === selectedId.value && (props.section !== 'all' || itemKind(x) === selectedKind.value))
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
    message.warning(t('tree.maxLevel', { n: MAX_LEVEL }))
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
    message.error(res.msg || t('common.createFailed'))
    return
  }
  message.success(t('tree.groupCreated'))
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
    message.error(res.msg || t('common.saveFailed'))
    return
  }
  message.success(t('tree.renamed'))
  await refresh()
}

function hasChildren(id: number): boolean {
  return groups.value.some((g) => g.p_id === id)
}

function askDelete(node: FlatGroup): void {
  if (hasChildren(node.id)) {
    dialog.error({ title: t('tree.cannotDelete'), content: t('tree.hasSubGroups'), positiveText: t('common.gotIt') })
    return
  }
  if ((itemsByGroup.value.get(node.id)?.length ?? 0) > 0) {
    dialog.error({ title: t('tree.cannotDelete'), content: t('tree.hasItems'), positiveText: t('common.gotIt') })
    return
  }
  dialog.warning({
    title: t('tree.deleteGroup'),
    content: t('tree.deleteGroupConfirm', { title: node.title }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = await deleteGroup(node.id)
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.deleteFailed'))
        return
      }
      message.success(t('common.deleted'))
      await refresh()
    },
  })
}

// ---- 目录上的「新建」菜单（子目录 / 接口） ----

/** 目录行「新建」菜单项：新建子目录始终可用，接口集合额外提供新建接口。 */
function groupActions(row: Row): DropdownOption[] {
  const options: DropdownOption[] = [
    { label: t('tree.newSub'), key: 'sub', disabled: row.level >= MAX_LEVEL },
  ]
  if (props.section === 'apis') options.push({ label: t('tree.newApi'), key: 'api' })
  if (props.section === 'all') {
    options.push({ label: t('tree.newApi'), key: 'api' })
    options.push({ label: t('tree.newDoc'), key: 'doc' })
  }
  return options
}

function onGroupAction(row: Row, key: string | number): void {
  const g = row.group
  if (!g) return
  if (key === 'sub') {
    startAddChild(row.level, g.id)
    return
  }
  if (key === 'api') createApiIn(g.id)
  if (key === 'doc') createDocIn(g.id)
}

/** 打开「新建接口」面板并预选该目录（合并后的工作台读 ?new=1&group= 预填）。 */
function createApiIn(groupId: number): void {
  void router.push({
    name: 'projectDocs',
    params: { projectId: props.projectId },
    query: { new: '1', group: String(groupId) },
  })
}

/** 跳到「新建文档」面板并预选该目录。 */
function createDocIn(groupId: number): void {
  void router.push({
    name: 'projectDocs',
    params: { projectId: props.projectId },
    query: { newDoc: '1', group: String(groupId) },
  })
}

// ---- 叶子删除（悬停在名称后出现） ----

/** 删除接口/文档（进回收站）；删掉当前选中项后回到空态占位。 */
function askDeleteItem(it: TreeItem): void {
  const label = itemKind(it) === 'api' ? t('tree.api') : t('tree.doc')
  dialog.warning({
    title: t('tree.deleteItem', { kind: label }),
    content: t('tree.deleteItemConfirm', { kind: label, title: it.title }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = itemKind(it) === 'api' ? await deleteApi(it.id) : await deleteDoc(it.id)
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.deleteFailed'))
        return
      }
      message.success(t('common.deleted'))
      if (selectedId.value === it.id && (props.section !== 'all' || selectedKind.value === itemKind(it))) clearSelection()
      await refresh()
    },
  })
}

/** 清空右侧选中（回到空态占位）。 */
function clearSelection(): void {
  const name = props.section === 'apis' ? 'projectApis' : 'projectDocs'
  void router.replace({ name, params: { projectId: props.projectId } })
}

// ---- 拖拽排序（分组同级重排/移动；叶子同级重排/跨组移动） ----

const canSort = computed(() => project.can(4))

type SortKind = 'group' | 'api' | 'doc' | 'mixed'
/** 叶子在容器里的键：api 与 doc 是两张表，主键会撞号，必须带类型前缀。 */
function leafKey(it: TreeItem): string {
  return `${itemKind(it)}:${it.id}`
}
interface DragNode { kind: 'group' | 'leaf'; key: string; container: number }
interface DropHint { key: string; pos: 'before' | 'after' | 'into' }
interface RelocateMove { src: number; dst: number; key: string; targetKey: string; pos: DropHint['pos'] }

const dragNode = ref<DragNode | null>(null)
const dropHint = ref<DropHint | null>(null)

function rowKey(row: Row): string {
  return (row.kind === 'group' ? 'g' : 'i') + (row.group?.id ?? row.item?.id)
}
function rowGroup(row: Row): FlatGroup | undefined {
  return row.kind === 'group' ? row.group : undefined
}
function rowItem(row: Row): TreeItem | undefined {
  return row.kind === 'item' ? row.item : undefined
}

/** 分组的有效父级：指向空标题/不存在分组的按根处理。 */
function parentOf(gid: number): number {
  const pid = groups.value.find((g) => g.id === gid)?.p_id ?? 0
  if (!pid || blankGroupIds.value.has(pid)) return 0
  return groups.value.some((g) => g.id === pid) ? pid : 0
}

/** 分组有序容器：parentId -> 同级分组键列表（统一用字符串键）。 */
function groupContainers(): Map<number, string[]> {
  const map = new Map<number, string[]>()
  for (const g of groups.value) {
    if (blankGroupIds.value.has(g.id)) continue
    const pid = parentOf(g.id)
    map.set(pid, [...(map.get(pid) ?? []), String(g.id)])
  }
  return map
}

/** 叶子有序容器：groupId -> 叶子 id 列表。 */
function leafContainers(): Map<number, string[]> {
  const map = new Map<number, string[]>()
  for (const it of items.value) {
    const gid = groupKey(it)
    map.set(gid, [...(map.get(gid) ?? []), leafKey(it)])
  }
  return map
}

/** 被拖行对应的节点；未分组行不可拖。 */
function dragNodeOf(row: Row): DragNode | null {
  const g = rowGroup(row)
  if (g) return g.id > 0 ? { kind: 'group', key: String(g.id), container: parentOf(g.id) } : null
  const it = rowItem(row)
  return it ? { kind: 'leaf', key: leafKey(it), container: groupKey(it) } : null
}

/** 依据鼠标在目标行的纵向位置推断落点：上下边缘为前/后插入，中间为移入（仅分组行）。 */
function dropPos(row: Row, e: DragEvent): DropHint['pos'] | null {
  const d = dragNode.value
  if (!d) return null
  const el = e.currentTarget as HTMLElement
  const ratio = (e.clientY - el.getBoundingClientRect().top) / (el.offsetHeight || 1)
  const g = rowGroup(row)
  if (g) {
    if (d.kind === 'leaf') return 'into'
    if (g.id <= 0 || d.kind === 'group' && String(g.id) === d.key) return null
    return ratio < 0.3 ? 'before' : ratio > 0.7 ? 'after' : 'into'
  }
  const it = rowItem(row)
  if (!it || d.kind !== 'leaf' || leafKey(it) === d.key) return null
  return ratio < 0.5 ? 'before' : 'after'
}

function onDragStart(row: Row, e: DragEvent): void {
  const node = canSort.value ? dragNodeOf(row) : null
  if (!node) {
    e.preventDefault()
    return
  }
  dragNode.value = node
  e.dataTransfer?.setData('text/plain', rowKey(row))
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

function onDragOver(row: Row, e: DragEvent): void {
  const pos = dropPos(row, e)
  if (!pos) {
    if (dropHint.value?.key === rowKey(row)) dropHint.value = null
    return
  }
  e.preventDefault()
  dropHint.value = { key: rowKey(row), pos }
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}

function onDrop(row: Row, e: DragEvent): void {
  e.preventDefault()
  const d = dragNode.value
  const hint = dropHint.value
  resetDrag()
  if (!d || !hint || hint.key !== rowKey(row)) return
  if (d.kind === 'group') void dropGroup(d, row, hint.pos)
  else void dropLeaf(d, row, hint.pos)
}

function resetDrag(): void {
  dragNode.value = null
  dropHint.value = null
}

/** 把 key 从 src 容器移到 dst 容器 targetKey 的前/后，或末尾（into）。 */
function relocate(containers: Map<number, string[]>, move: RelocateMove): void {
  const from = containers.get(move.src) ?? []
  const at = from.indexOf(move.key)
  if (at >= 0) from.splice(at, 1)
  containers.set(move.src, from)
  const to = containers.get(move.dst) ?? []
  const ti = move.pos !== 'into' && move.targetKey ? to.indexOf(move.targetKey) : -1
  to.splice(ti < 0 ? to.length : move.pos === 'before' ? ti : ti + 1, 0, move.key)
  containers.set(move.dst, to)
}

/** 汇总被改动容器的新顺序（priority 即索引）。 */
function sortPayload(containers: Map<number, string[]>, changed: number[], kind: SortKind): { kind: SortKind; project_id: number; items: SortItem[] } {
  const items: SortItem[] = []
  for (const cid of changed) {
    (containers.get(cid) ?? []).forEach((key, i) => {
      // 叶子键形如 `api:7`；分组键是纯数字（沿用 req.Kind=group）。
      const cut = key.indexOf(':')
      if (cut < 0) {
        items.push({ id: Number(key), parent_id: cid, priority: i })
        return
      }
      items.push({ id: Number(key.slice(cut + 1)), parent_id: cid, priority: i, kind: key.slice(0, cut) as 'api' | 'doc' })
    })
  }
  return { kind, project_id: props.projectId, items }
}

async function dropLeaf(d: DragNode, row: Row, pos: DropHint['pos']): Promise<void> {
  const g = rowGroup(row)
  const it = rowItem(row)
  const dst = g ? g.id : it ? groupKey(it) : 0
  const targetKey = it ? leafKey(it) : ''
  const containers = leafContainers()
  relocate(containers, { src: d.container, dst, key: d.key, targetKey, pos })
  const changed = d.container === dst ? [dst] : [d.container, dst]
  // 混排工作台里同一批可能既有接口又有文档，逐条带 kind（服务端按 mixed 处理）。
  const kind: SortKind = props.section === 'all'
    ? 'mixed'
    : props.section === 'apis' ? 'api' : 'doc'
  await submitSort(sortPayload(containers, changed, kind))
}

async function dropGroup(d: DragNode, row: Row, pos: DropHint['pos']): Promise<void> {
  const g = rowGroup(row)
  if (!g) return
  const dst = pos === 'into' ? g.id : parentOf(g.id)
  if (d.kind === 'group' && String(dst) === d.key) return
  const containers = groupContainers()
  relocate(containers, { src: d.container, dst, key: d.key, targetKey: pos === 'into' ? '' : String(g.id), pos })
  const changed = d.container === dst ? [dst] : [d.container, dst]
  await submitSort(sortPayload(containers, changed, 'group'))
}

async function submitSort(payload: { kind: SortKind; project_id: number; items: SortItem[] }): Promise<void> {
  if (!payload.items.length) return
  const res = await sortNodes(payload)
  if (res.code !== ErrCode.Success) {
    message.error(res.msg || t('tree.sortFailed'))
    return
  }
  await refresh()
}

/** 行落点样式；无落点返回空对象。 */
function dropClass(row: Row): Record<string, boolean> {
  const h = dropHint.value
  if (!h || h.key !== rowKey(row)) return {}
  return { 'gn--drop-before': h.pos === 'before', 'gn--drop-after': h.pos === 'after', 'gn--drop-into': h.pos === 'into' }
}

/** 该行是否为当前拖动源。 */
function isDragging(row: Row): boolean {
  const d = dragNode.value
  if (!d) return false
  if (d.kind === 'group') return String(rowGroup(row)?.id ?? '') === d.key
  const it = rowItem(row)
  return !!it && leafKey(it) === d.key
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
      <span class="gtree-title">{{ section === 'apis' ? t('tree.apiSet') : section === 'all' ? t('tree.contentSet') : t('tree.docSet') }}</span>
      <span v-if="items.length" class="gtree-count">{{ items.length }}</span>
      <n-button
        quaternary
        size="tiny"
        class="gtree-act"
        :title="allOpen ? t('tree.collapseAll') : t('tree.expandAll')"
        @click="toggleAll"
      >
        <template #icon><n-icon :component="allOpen ? ContractOutline : ExpandOutline" /></template>
      </n-button>
      <n-button v-if="canCreate" quaternary size="tiny" class="gtree-act" :title="t('tree.newDir')" @click="startAddRoot">
        <template #icon><n-icon :component="AddOutline" /></template>
      </n-button>
      <n-button v-if="collapsible" quaternary size="tiny" class="gtree-act" :title="t('tree.collapseSidebar')" @click="emit('collapse')">
        <template #icon><n-icon :component="ChevronBackOutline" /></template>
      </n-button>
    </div>

    <div class="gtree-search">
      <n-input v-model:value="keyword" size="small" clearable :placeholder="section === 'apis' ? t('tree.searchApis') : section === 'all' ? t('tree.searchAll') : t('tree.searchDocs')">
        <template #prefix><n-icon :component="SearchOutline" /></template>
      </n-input>
    </div>

    <div class="gtree-list">
      <div v-if="addingParent === 0" class="gn gn--add" @click.stop>
        <span class="gn-caret" />
        <n-input v-model:value="addTitle" size="tiny" :placeholder="t('tree.groupName')" @keyup.enter="submitAdd" @blur="submitAdd" />
      </div>

      <template v-for="row in rows" :key="(row.kind === 'group' ? 'g' : 'i') + (row.group?.id ?? row.item?.id)">
        <!-- 分组行 -->
        <div
          v-if="row.kind === 'group' && row.group"
          class="gn gn--group"
          :class="[dropClass(row), { 'gn--dragging': isDragging(row) }]"
          :style="{ paddingLeft: 6 + (row.level - 1) * 14 + 'px' }"
          :draggable="canSort && row.group.id > 0"
          @click="toggle(row.group.id)"
          @dragstart="onDragStart(row, $event)"
          @dragover="onDragOver(row, $event)"
          @drop="onDrop(row, $event)"
          @dragend="resetDrag"
        >
          <button
            v-if="groupExpandable(row.group)"
            class="gn-caret"
            :aria-label="isOpen(row.group) ? t('tree.collapse') : t('tree.expand')"
            @click.stop="toggle(row.group.id)"
          >
            <n-icon :component="ChevronForwardOutline" :class="{ 'is-open': isOpen(row.group) }" />
          </button>
          <span v-else class="gn-caret" />

          <n-icon
            class="gn-folder"
            :component="groupExpandable(row.group) && isOpen(row.group) ? FolderOpenOutline : FolderOutline"
          />

          <n-input
            v-if="row.group.id > 0 && renamingId === row.group.id"
            v-model:value="renameTitle"
            size="tiny"
            @click.stop
            @keyup.enter="submitRename"
            @blur="submitRename"
          />
          <!-- B19：文档分段的分组标题可点 = 按该分组过滤右侧列表（接口段仍只折展） -->
          <span
            v-else-if="props.section === 'docs'"
            class="gn-name ellipsis gn-name--filter"
            :title="row.group.title"
            data-testid="docs.treeGroup"
            @click.stop="filterByGroup(row.group.id)"
          >{{ row.group.title }}</span>
          <span v-else class="gn-name ellipsis" :title="row.group.title">{{ row.group.title }}</span>

          <span v-if="renamingId !== row.group.id && row.group.id > 0" class="gn-actions">
            <n-dropdown
              v-if="canCreate"
              trigger="click"
              placement="bottom-start"
              :options="groupActions(row)"
              @select="(key) => onGroupAction(row, key)"
            >
              <button :title="t('tree.new')" @click.stop>
                <n-icon :component="AddOutline" />
              </button>
            </n-dropdown>
            <button v-if="canUpdate" :title="t('tree.rename')" @click.stop="startRename(row.group.id, row.group.title)">
              <n-icon :component="PencilOutline" />
            </button>
            <button v-if="canDelete" :title="t('tree.delDir')" @click.stop="askDelete(row.group)">
              <n-icon :component="TrashOutline" />
            </button>
          </span>
        </div>

        <!-- 分组行内新建子分组录入 -->
        <div
          v-if="row.kind === 'group' && row.group && row.group.id > 0 && addingParent === row.group.id"
          class="gn gn--add"
          :style="{ paddingLeft: 6 + row.level * 14 + 16 + 'px' }"
          @click.stop
        >
          <span class="gn-caret" />
          <n-input v-model:value="addTitle" size="tiny" :placeholder="t('tree.subGroupName')" @keyup.enter="submitAdd" @blur="submitAdd" />
        </div>

        <!-- 叶子行：接口（方法 + 名称）/ 文档（标题） -->
        <div
          v-else-if="row.kind === 'item' && row.item"
          class="gn gn--leaf"
          :class="[dropClass(row), { 'gn--dragging': isDragging(row), 'gn--active': selectedId === row.item.id && (section !== 'all' || selectedKind === itemKind(row.item)) && !trashActive }]"
          :style="{ paddingLeft: 6 + (row.level - 1) * 14 + 16 + 'px' }"
          :draggable="canSort"
          @click="goItem(row.item)"
          @dragstart="onDragStart(row, $event)"
          @dragover="onDragOver(row, $event)"
          @drop="onDrop(row, $event)"
          @dragend="resetDrag"
        >
          <method-tag v-if="itemKind(row.item) === 'api'" :method="row.item.method ?? ''" class="gn-method" />
          <img v-else-if="isImageIcon(row.item.icon)" class="gn-icon-img" :src="row.item.icon ?? ''" alt="" />
          <span v-else class="gn-icon">{{ row.item.icon || DEFAULT_DOC_ICON }}</span>
          <span class="gn-name ellipsis" :title="row.item.title">{{ row.item.title }}</span>

          <!-- 删除收在名称后：悬停出现 -->
          <span v-if="canDelete && !trashActive" class="gn-actions">
            <button :title="itemKind(row.item) === 'api' ? t('tree.delApi') : t('tree.delDoc')" @click.stop="askDeleteItem(row.item)">
              <n-icon :component="TrashOutline" />
            </button>
          </span>
        </div>
      </template>

      <div v-if="!rows.length" class="gtree-empty meta">{{ searching ? t('search.empty') : t('tree.noContent') }}</div>
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

/* 计数：圆圈 + 居中数字（与标题文字对齐，配色同品牌绿） */
.gtree-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 17px;
  height: 17px;
  margin-right: auto;
  padding: 0 4px;
  border: 1px solid var(--app-accent);
  border-radius: 999px;
  background: rgba(24, 160, 88, 0.1);
  color: var(--app-accent);
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
}

.gtree-act {
  color: var(--app-muted);
}

.gtree-act:hover {
  color: var(--app-accent);
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
  user-select: none;
}

/* 拖拽反馈：拖动源半透明 / 移入高亮 / 前后插入线 */
.gn--dragging {
  opacity: 0.5;
}

.gn--drop-into,
.gn--drop-into:hover {
  background: #d8f0e2;
  box-shadow: inset 0 0 0 1px var(--app-accent);
}

.gn--drop-before {
  box-shadow: inset 0 2px 0 0 var(--app-accent);
}

.gn--drop-after {
  box-shadow: inset 0 -2px 0 0 var(--app-accent);
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
  flex: 0 0 52px;
}

.gn-dot {
  flex: 0 0 auto;
  width: 4px;
  height: 4px;
  margin-right: 2px;
  border-radius: 50%;
  background: #b9bfc6;
}

/* 文档图标（emoji 或图片） */
.gn-icon {
  flex: 0 0 auto;
  width: 16px;
  font-size: 13px;
  line-height: 1;
  text-align: center;
}

.gn-icon-img {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  border-radius: 3px;
  object-fit: cover;
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
  color: #6b7280;
  font-size: 14px;
  cursor: pointer;
}

.gn--group:hover .gn-caret {
  color: #26292e;
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
