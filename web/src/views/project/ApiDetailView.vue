<script setup lang="ts">
// 接口详情（Postman 请求编辑器样式）：名称栏 + URL 栏，下方按「请求头部 / 请求参数 / 返回参数 / 响应样例 / 基本信息(有值时) / 文档」垂直排列。
// 右侧悬浮大纲（收起为小格子、悬停展开、点击跳段）与长页「返回顶部」；文档一节用 MdEditor/MdPreview 承载 Markdown。
// 请求头直接以 ApiParam[] 承载编辑（与 ApiParamTable 同构），只在载入/保存时与 ApiHeader[] 互转，避免中间态丢输入。
import { deleteApiSample, getApi, getCode, listApiSamples, listGroups, saveApiSample, updateApi, type ApiReturnSample } from '@/api/content'
import ApiParamTable from '@/components/ApiParamTable.vue'
import EnvPicker from '@/components/EnvPicker.vue'
import JsonBlock from '@/components/JsonBlock.vue'
import MethodTag from '@/components/MethodTag.vue'
import RevisionHistory from '@/components/RevisionHistory.vue'
import TryItPanel from '@/components/TryItPanel.vue'
import VarText from '@/components/VarText.vue'
import { useMdUpload } from '@/composables/md-upload'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { useVariableStore } from '@/stores/variable'
import { ErrCode } from '@/types/api'
import type { ApiHeader, ApiParam, Group, ProxyAuth } from '@/types/entities'
import { headersToParams, paramsToHeaders } from '@/utils/api-params'
import {
    apiToMarkdown, apiToStandaloneHtml, downloadText, safeName, type ApiExportInput,
} from '@/utils/export-doc'
import {
    ArrowUpOutline, CodeSlashOutline, CopyOutline, CreateOutline, DownloadOutline, PlayOutline,
    RefreshOutline, TimeOutline,
} from '@vicons/ionicons5'
import { MdEditor, MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import 'md-editor-v3/lib/style.css'
import {
    NButton, NDropdown, NIcon, NInput, NInputNumber, NModal, NSelect, NSpin, NTag, useDialog, useMessage,
    type DropdownOption,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const variableStore = useVariableStore()
const { t } = useI18n()

const props = defineProps<{ apiId?: number }>()
const emit = defineEmits<{ (e: 'changed'): void }>()

const projectId = useNumParam('projectId')
const apiId = computed(() => props.apiId ?? (Number(route.params.apiId) || 0))

const loading = ref(false)
const saving = ref(false)
const editing = ref(route.query.edit === '1')
const tryItShow = ref(false)
const groups = ref<Group[]>([])

// ---- 历史版本（RM-05） ----
const historyShow = ref(false)

const groupId = ref(0)
const groupIdSecond = ref(0)
const form = reactive({
  api_name: '', http_method_type: 'GET', url: '', protocol_type: 'HTTP',
  description: '', develop_language: '', http_return_type: 'JSON',
  status: 2, version: '',
})
/** 请求头以参数行形态编辑（表格直接改这份数组），保存时再转回 ApiHeader[]。 */
const headerRows = ref<ApiParam[]>([])
const reqParams = ref<ApiParam[]>([])
const resParams = ref<ApiParam[]>([])
const sample = ref('')

/**
 * 客户端同步过来的富字段（请求体/认证/请求设置/脚本/变量/断言/响应内容类型/请求体备注）：
 * 均以原始 JSON/文本串保存，Web 只读展示；保存时必须原样回传，否则整条覆盖会清空服务端这些列。
 */
const ext = reactive({
  http_request_body_type: '', http_request_body: '', http_request_body_form: '',
  http_request_body_descs: '', http_auth: '', request_settings: '',
  http_script: '', script_vars: '', script_asserts: '', http_return_content_type: '',
})

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].map((m) => ({ label: m, value: m }))
const statusOptions = computed(() => [
  { label: t('api.statusDraft'), value: 1 },
  { label: t('api.statusPublished'), value: 2 },
  { label: t('api.statusDeprecated'), value: 3 },
])
const statusLabel = computed(() => {
  const map: Record<number, string> = {
    1: t('api.statusDraft'), 2: t('api.statusPublished'), 3: t('api.statusDeprecated'),
  }
  return map[form.status] || t('api.statusPublished')
})
const statusTagType = computed<'default' | 'success' | 'warning' | 'error'>(() => {
  if (form.status === 1) return 'warning'
  if (form.status === 3) return 'error'
  return 'success'
})

/** 载入时的 update_time：保存时作为乐观锁期望值回传。 */
const loadedUpdateTime = ref('')
const groupOptions = computed(() => [
  { label: t('tree.ungrouped'), value: 0 },
  ...groups.value.map((g) => ({ label: g.title, value: g.id })),
])
const groupName = computed(() => groups.value.find((g) => g.id === groupId.value)?.title || t('tree.ungrouped'))

const canUpdate = computed(() => project.can(4))

/** 平台统一失败响应示例（与 doc/api_docs/common/error-codes.yml 约定一致）。 */
const failedSample = JSON.stringify({ code: 22, msg: '参数错误', data: null })

const onUploadImg = useMdUpload()

// ---- 大纲（收起为小格子 / 悬停展开 / 点击跳段）与返回顶部 ----
const bodyRef = ref<HTMLElement | null>(null)
const activeSec = ref('headers')
const bodyScrollTop = ref(0)
const showTop = computed(() => bodyScrollTop.value > 240)

/** 当前可见的章节（与模板中的 data-sec 一一对应）。 */
const sections = computed(() => {
  const list = [
    { key: 'headers', label: t('param.headerTitle') },
    { key: 'params', label: t('param.requestTitle') },
  ]
  // 客户端富字段：查看态有内容才出现；编辑态常显（可从零补齐），与模板 data-sec 一一对应。
  if (!editing.value && hasBody.value) list.push({ key: 'body', label: t('detail.secBody') })
  if (editing.value) list.push({ key: 'body', label: t('detail.secBody') })
  if (!editing.value && hasAuth.value) list.push({ key: 'auth', label: t('detail.secAuth') })
  if (editing.value) list.push({ key: 'auth', label: t('detail.secAuth') })
  list.push({ key: 'result', label: t('param.responseTitle') })
  list.push({ key: 'sample', label: t('detail.secSample') })
  if (!editing.value && hasSettings.value) list.push({ key: 'settings', label: t('detail.secSettings') })
  if (editing.value) list.push({ key: 'settings', label: t('detail.secSettings') })
  if (!editing.value && hasScript.value) list.push({ key: 'script', label: t('detail.secScript') })
  if (!editing.value && form.develop_language) list.push({ key: 'info', label: t('detail.secInfo') })
  list.push({ key: 'doc', label: t('detail.secDoc') })
  return list
})

/** 滚动时同步「返回顶部」显隐与大纲高亮。 */
function onBodyScroll(): void {
  const body = bodyRef.value
  if (!body) return
  bodyScrollTop.value = body.scrollTop
  const bodyTop = body.getBoundingClientRect().top
  let current = sections.value[0]?.key ?? ''
  for (const s of sections.value) {
    const el = body.querySelector<HTMLElement>(`[data-sec="${s.key}"]`)
    if (el && el.getBoundingClientRect().top - bodyTop <= 24) current = s.key
  }
  activeSec.value = current
}

function scrollToSection(key: string): void {
  const body = bodyRef.value
  const el = body?.querySelector<HTMLElement>(`[data-sec="${key}"]`)
  if (!body || !el) return
  const top = el.getBoundingClientRect().top - body.getBoundingClientRect().top + body.scrollTop - 8
  body.scrollTo({ top, behavior: 'smooth' })
}

function scrollToTop(): void {
  bodyRef.value?.scrollTo({ top: 0, behavior: 'smooth' })
}

/** 宽容解析 JSON 数组列，非法时返回空数组。 */
function parseArray<T>(raw: string | undefined): T[] {
  if (!raw) return []
  try {
    const v = JSON.parse(raw) as unknown
    return Array.isArray(v) ? (v as T[]) : []
  } catch {
    return []
  }
}

/** 美化 JSON 文本，失败则原样展示。 */
function pretty(raw: string | undefined): string {
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

/** 宽容解析 JSON 对象列，非法/数组时返回空对象（富字段以 JSON 串存储）。 */
function parseObject(raw: string | undefined): Record<string, unknown> {
  if (!raw) return {}
  try {
    const v = JSON.parse(raw) as unknown
    return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}

/** 取字符串字段（缺省空串）。 */
function str(o: Record<string, unknown>, k: string): string {
  const v = o[k]
  return typeof v === 'string' ? v : ''
}

/** 三态布尔 smallint（-1 未设置 / 0 关 / 1 开）→ 可读文案。 */
function triLabel(v: unknown): string {
  if (v === 1) return t('detail.triOn')
  if (v === 0) return t('detail.triOff')
  return t('detail.triUnset')
}

/** form/multipart 请求体行。 */
interface BodyFormRow { name?: string; value?: string; enabled?: boolean; description?: string; type?: string }
interface NamedVal { name?: string; value?: string; expr?: string; enabled?: boolean }

// ---- 富字段编辑（⑤）：进编辑态时从 ext 串解析，保存时序列化回 ext（未知键保留，全空则清空串） ----

/** 客户端请求体 mode 枚举（同步键 bodyType 的取值域）。 */
const BODY_TYPES = ['none', 'json', 'text', 'form', 'multipart']
/** 客户端认证类型枚举（inherit=沿用上级）。 */
const AUTH_TYPES = ['inherit', 'none', 'basic', 'bearer', 'apikey']
/** 返回内容类型候选（select 允许自由填写）。 */
const RET_CONTENT_TYPES = ['application/json', 'application/xml', 'text/plain', 'text/html']

const extEdit = reactive({
  bodyType: 'none',
  bodyRaw: '',
  bodyForm: [] as BodyFormRow[],
  bodyDescs: [] as { path: string; note: string }[],
  authType: '',
  authUser: '', authPass: '', authToken: '', authKey: '', authValue: '', authIn: 'header',
  authBase: {} as Record<string, unknown>,
  timeout: null as number | null,
  maxRedirects: null as number | null,
  followRedirects: -1,
  insecureSsl: -1,
  encodeUrl: '' as '' | 'yes' | 'no',
  settingsBase: {} as Record<string, unknown>,
  retContentType: '',
})

// 在线调试面板的初始数据：编辑态用未保存的 extEdit，查看态用已保存的 ext。
const debugBodyType = computed(() => (editing.value ? extEdit.bodyType : (ext.http_request_body_type || 'none')))
const debugBodyRaw = computed(() => (editing.value ? extEdit.bodyRaw : ext.http_request_body))
const debugBodyForm = computed<BodyFormRow[]>(() =>
  editing.value ? extEdit.bodyForm : parseArray<BodyFormRow>(ext.http_request_body_form))
const debugAuth = computed<ProxyAuth | null>(() => {
  if (editing.value) {
    if (!extEdit.authType || extEdit.authType === 'inherit' || extEdit.authType === 'none') return null
    return {
      type: extEdit.authType, username: extEdit.authUser, password: extEdit.authPass,
      token: extEdit.authToken, key: extEdit.authKey, value: extEdit.authValue, in: extEdit.authIn,
    }
  }
  const a = parseObject(ext.http_auth)
  const type = str(a, 'type')
  if (!type || type === 'inherit' || type === 'none') return null
  return {
    type, username: str(a, 'username'), password: str(a, 'password'), token: str(a, 'token'),
    key: str(a, 'key'), value: str(a, 'value'), in: str(a, 'in'),
  }
})

const bodyTypeOptions = BODY_TYPES.map((v) => ({ label: v, value: v }))
const authTypeOptions = AUTH_TYPES.map((v) => ({ label: v, value: v }))
const authInOptions = [{ label: 'header', value: 'header' }, { label: 'query', value: 'query' }]
const triStateOptions = computed(() => [
  { label: t('detail.triUnset'), value: -1 },
  { label: t('detail.triOn'), value: 1 },
  { label: t('detail.triOff'), value: 0 },
])
const encodeUrlOptions = computed(() => [
  { label: t('detail.triUnset'), value: '' },
  { label: t('detail.triOn'), value: 'yes' },
  { label: t('detail.triOff'), value: 'no' },
])
const retContentTypeOptions = RET_CONTENT_TYPES.map((v) => ({ label: v, value: v }))

/** 从 ext 原始串解析出编辑态（startEdit 调用；解析失败按空处理，绝不 throw 阻断编辑）。 */
function parseExtEdit(): void {
  extEdit.bodyType = ext.http_request_body_type || 'none'
  extEdit.bodyRaw = ext.http_request_body
  extEdit.bodyForm = parseArray<BodyFormRow>(ext.http_request_body_form)
  extEdit.bodyDescs = Object.entries(parseObject(ext.http_request_body_descs))
    .map(([path, note]) => ({ path, note: String(note ?? '') }))
  const a = parseObject(ext.http_auth)
  extEdit.authBase = a
  extEdit.authType = str(a, 'type')
  extEdit.authUser = str(a, 'username')
  extEdit.authPass = str(a, 'password')
  extEdit.authToken = str(a, 'token')
  extEdit.authKey = str(a, 'key')
  extEdit.authValue = str(a, 'value')
  extEdit.authIn = str(a, 'in') || 'header'
  const s = parseObject(ext.request_settings)
  extEdit.settingsBase = s
  extEdit.timeout = typeof s.timeout === 'number' && s.timeout > 0 ? s.timeout : null
  extEdit.maxRedirects = typeof s.maxRedirects === 'number' && s.maxRedirects > 0 ? s.maxRedirects : null
  extEdit.followRedirects = typeof s.followRedirects === 'number' ? s.followRedirects : -1
  extEdit.insecureSsl = typeof s.insecureSsl === 'number' ? s.insecureSsl : -1
  extEdit.encodeUrl = typeof s.encodeUrl === 'boolean' ? (s.encodeUrl ? 'yes' : 'no') : ''
  extEdit.retContentType = ext.http_return_content_type
}

/** 编辑态序列化回 ext 串（save 前调用；语义：全空 → 空串清空对应列）。 */
function serializeExtEdit(): void {
  const rows = extEdit.bodyForm.filter((r) => (r.name ?? '').trim())
  const descs = extEdit.bodyDescs.filter((d) => d.path.trim())
  const bt = extEdit.bodyType
  if (bt === 'none' && !extEdit.bodyRaw.trim() && !rows.length && !descs.length) {
    ext.http_request_body_type = ''
    ext.http_request_body = ''
    ext.http_request_body_form = ''
    ext.http_request_body_descs = ''
  } else {
    ext.http_request_body_type = bt
    ext.http_request_body = bt === 'json' || bt === 'text' ? extEdit.bodyRaw : ''
    ext.http_request_body_form = bt === 'form' || bt === 'multipart'
      ? JSON.stringify(rows.map((r) => ({
        name: r.name, value: r.value ?? '', enabled: r.enabled !== false,
        description: r.description ?? '', type: r.type ?? 'text',
      })))
      : ''
    ext.http_request_body_descs = descs.length
      ? JSON.stringify(Object.fromEntries(descs.map((d) => [d.path.trim(), d.note.trim()])))
      : ''
  }

  const at = extEdit.authType
  if (!at) {
    ext.http_auth = ''
  } else {
    const a = { ...extEdit.authBase }
    for (const k of ['type', 'username', 'password', 'token', 'key', 'value', 'in']) delete a[k]
    a.type = at
    if (at === 'basic') { a.username = extEdit.authUser; a.password = extEdit.authPass }
    if (at === 'bearer') a.token = extEdit.authToken
    if (at === 'apikey') { a.key = extEdit.authKey; a.value = extEdit.authValue; a.in = extEdit.authIn || 'header' }
    ext.http_auth = JSON.stringify(a)
  }

  const s: Record<string, unknown> = { ...extEdit.settingsBase }
  for (const k of ['timeout', 'maxRedirects', 'followRedirects', 'insecureSsl', 'encodeUrl']) delete s[k]
  if (extEdit.timeout && extEdit.timeout > 0) s.timeout = extEdit.timeout
  if (extEdit.maxRedirects && extEdit.maxRedirects > 0) s.maxRedirects = extEdit.maxRedirects
  if (extEdit.followRedirects !== -1) s.followRedirects = extEdit.followRedirects
  if (extEdit.insecureSsl !== -1) s.insecureSsl = extEdit.insecureSsl
  if (extEdit.encodeUrl !== '') s.encodeUrl = extEdit.encodeUrl === 'yes'
  ext.request_settings = Object.keys(s).length ? JSON.stringify(s) : ''

  ext.http_return_content_type = extEdit.retContentType.trim()
}

const bodyFormRows = computed<BodyFormRow[]>(() => parseArray<BodyFormRow>(ext.http_request_body_form))
/** JSON 体字段备注（path → 备注），来自客户端表格视图。 */
const bodyDescRows = computed(() =>
  Object.entries(parseObject(ext.http_request_body_descs))
    .map(([path, note]) => ({ path, note: typeof note === 'string' ? note : String(note) })))
const authObj = computed(() => parseObject(ext.http_auth))
const settingsObj = computed(() => parseObject(ext.request_settings))
const scriptObj = computed(() => parseObject(ext.http_script))
const varsRows = computed<NamedVal[]>(() => parseArray<NamedVal>(ext.script_vars))
const assertRows = computed<NamedVal[]>(() => parseArray<NamedVal>(ext.script_asserts))

/** 各富字段段是否有内容（仅查看态、且非空时才出现在大纲/正文）。 */
const hasBody = computed(() =>
  (!!ext.http_request_body_type && ext.http_request_body_type !== 'none') ||
  !!ext.http_request_body || bodyFormRows.value.length > 0 || bodyDescRows.value.length > 0)
const hasAuth = computed(() => !!str(authObj.value, 'type'))
const hasSettings = computed(() =>
  !!ext.request_settings && Object.keys(settingsObj.value).length > 0)
const hasScript = computed(() =>
  !!(str(scriptObj.value, 'preRequest') || str(scriptObj.value, 'postResponse')) ||
  varsRows.value.length > 0 || assertRows.value.length > 0)

/** 「返回参数」标题旁徽标：根类型 + 响应内容类型（若客户端设过）。 */
const resultTitleExtra = computed(() =>
  [form.http_return_type, ext.http_return_content_type].filter(Boolean).join(' · '))


/** 加载失败或详情不可用时退回列表（空态占位）。 */
function goPlaceholder(): void {
  void router.replace({ name: 'projectApis', params: { projectId: projectId.value } })
}

/** 载入详情。返回是否成功拿到数据（失败已提示并退回列表）。 */
async function load(): Promise<boolean> {
  if (!apiId.value) return false
  loading.value = true
  try {
    const res = await getApi(apiId.value)
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('detail.loadFailed'))
      goPlaceholder()
      return false
    }
    const a = res.data
    // 并发编辑保护：保存时回传载入时间，服务端据此判断是否被他人改过。
    loadedUpdateTime.value = a.update_time
    groupId.value = a.group_id
    groupIdSecond.value = a.group_id_second
    Object.assign(form, {
      api_name: a.api_name,
      http_method_type: a.http_method_type || 'GET',
      url: a.url,
      protocol_type: a.protocol_type || 'HTTP',
      description: a.description,
      develop_language: a.develop_language ?? '',
      http_return_type: a.http_return_type || 'JSON',
      status: a.status || 2,
      version: a.version ?? '',
    })
    headerRows.value = headersToParams(parseArray<ApiHeader>(a.http_request_header))
    reqParams.value = parseArray<ApiParam>(a.http_request_params)
    // 富字段原样存入 ext（只读展示 + 保存回传）
    Object.assign(ext, {
      http_request_body_type: a.http_request_body_type ?? '',
      http_request_body: a.http_request_body ?? '',
      http_request_body_form: a.http_request_body_form ?? '',
      http_request_body_descs: a.http_request_body_descs ?? '',
      http_auth: a.http_auth ?? '',
      request_settings: a.request_settings ?? '',
      http_script: a.http_script ?? '',
      script_vars: a.script_vars ?? '',
      script_asserts: a.script_asserts ?? '',
      http_return_content_type: a.http_return_content_type ?? '',
    })
    resParams.value = parseArray<ApiParam>(a.http_return_params)
    sample.value = pretty(a.http_return_sample)
    void loadSamples()
    return true
  } finally {
    loading.value = false
  }
}

/** 刷新当前接口：重新拉取详情与变量解析；编辑态先确认丢弃未保存改动。 */
function refreshApi(): void {
  const doRefresh = async (): Promise<void> => {
    const ok = await load()
    if (!ok) return
    await loadVariables()
    draft = null
    message.success(t('detail.refreshed'))
  }
  if (editing.value) {
    dialog.warning({
      title: t('detail.refreshTitle'),
      content: t('detail.refreshDiscard'),
      positiveText: t('common.confirm'),
      negativeText: t('common.cancel'),
      onPositiveClick: doRefresh,
    })
    return
  }
  void doRefresh()
}

async function loadGroups(): Promise<void> {
  const res = await listGroups(projectId.value, 1)
  if (res.code === ErrCode.Success && res.data) groups.value = res.data
}

/** 变量解析表：接口或当前环境变化时重算（结果缓存在 store，左树徽标与其同源）。 */
async function loadVariables(): Promise<void> {
  if (!projectId.value || !apiId.value) return
  await variableStore.resolve(projectId.value, apiId.value)
}

watch(apiId, () => void load())
watch([apiId, () => variableStore.currentEnv[projectId.value]], () => void loadVariables(), { immediate: true })
onMounted(() => {
  void loadGroups()
  void load()
})

/** 退出编辑后清掉地址栏 edit 标记，避免刷新又回到编辑态。 */
function clearEditQuery(): void {
  if (!route.query.edit) return
  void router.replace({ name: 'projectApis', params: { projectId: projectId.value }, query: { api: String(apiId.value) } })
}

/** 进入编辑时的状态快照，供「取消」即时回滚（避免等待重新拉取）。 */
interface ApiDraft {
  groupId: number
  groupIdSecond: number
  form: typeof form
  headerRows: ApiParam[]
  reqParams: ApiParam[]
  resParams: ApiParam[]
  sample: string
}
let draft: ApiDraft | null = null

function startEdit(): void {
  draft = {
    groupId: groupId.value,
    groupIdSecond: groupIdSecond.value,
    form: { ...form },
    headerRows: headerRows.value.map((r) => ({ ...r })),
    reqParams: reqParams.value.map((p) => ({ ...p })),
    resParams: resParams.value.map((p) => ({ ...p })),
    sample: sample.value,
  }
  parseExtEdit()
  editing.value = true
}

function cancelEdit(): void {
  if (draft) {
    groupId.value = draft.groupId
    groupIdSecond.value = draft.groupIdSecond
    Object.assign(form, draft.form)
    headerRows.value = draft.headerRows
    reqParams.value = draft.reqParams
    resParams.value = draft.resParams
    sample.value = draft.sample
    draft = null
  } else {
    // 直接以 ?edit=1 进入编辑时没有快照，回退到重新拉取
    void load()
  }
  editing.value = false
  clearEditQuery()
}

async function save(): Promise<void> {
  let sampleVal: unknown = ''
  if (sample.value.trim()) {
    try {
      sampleVal = JSON.parse(sample.value)
    } catch {
      message.error(t('api.sampleInvalid'))
      return
    }
  }
  saving.value = true
  try {
    serializeExtEdit()
    const res = await updateApi({
      id: apiId.value,
      project_id: projectId.value,
      group_id: groupId.value,
      group_id_second: groupIdSecond.value,
      ...form,
      // 富字段原样回传：整条覆盖式更新，缺省会把同步来的这些列清空。
      ...ext,
      http_request_header: paramsToHeaders(headerRows.value),
      http_request_params: reqParams.value,
      http_return_params: resParams.value,
      http_return_sample: sampleVal,
      // 乐观锁：他人已保存时服务端会拒绝覆盖。
      expected_update_time: loadedUpdateTime.value,
    })
    if (res.code !== ErrCode.Success) {
      message.error(res.msg || t('common.saveFailed'))
      // 冲突：提示并可直接载入最新内容（避免继续在旧内容上编辑）。
      if ((res.msg || '').includes('已被他人修改')) {
        dialog.warning({
          title: t('detail.conflictTitle'),
          content: t('detail.conflictContent', { msg: res.msg }),
          positiveText: t('detail.conflictReload'),
          negativeText: t('detail.conflictLater'),
          onPositiveClick: async () => {
            await load()
            editing.value = false
            clearEditQuery()
          },
        })
      }
      return
    }
    message.success(t('common.saved'))
    draft = null
    editing.value = false
    clearEditQuery()
    emit('changed')
    await load()
  } finally {
    saving.value = false
  }
}

/** 响应 JSON 生成对应字段后，样例框为空则顺带填入，避免两处内容不一致。 */
function onSampleJson(text: string): void {
  if (!sample.value.trim()) sample.value = text.trim()
}

// ---- 响应示例（多示例，与客户端保存的响应示例互通） ----
const samples = ref<ApiReturnSample[]>([])
const sampleId = ref(0)
const sampleText = ref('')
const sampleSaving = ref(false)
const showSampleModal = ref(false)
const sampleName = ref('')

const sampleOptions = computed(() => samples.value.map((s) => ({ label: `${s.name} (${s.status})`, value: s.id })))

async function loadSamples(): Promise<void> {
  if (!apiId.value) return
  const res = await listApiSamples(apiId.value)
  if (res.code !== ErrCode.Success || !res.data) return
  samples.value = res.data
  pickSample(samples.value[0]?.id ?? 0)
}

/** 切换选中的示例（编辑中的正文直接丢弃，选示例即所见即所得）。 */
function pickSample(id: number): void {
  sampleId.value = id
  const cur = samples.value.find((x) => x.id === id)
  sampleText.value = cur ? pretty(cur.sample_text) : ''
}

/** 新建示例：以主样例当前内容为初始正文。 */
async function createSample(): Promise<void> {
  const name = sampleName.value.trim()
  if (!name) return
  const res = await saveApiSample({ api_id: apiId.value, name, sample_text: sample.value, status: 200 })
  if (res.code !== ErrCode.Success || !res.data) {
    message.error(res.msg || t('common.saveFailed'))
    return
  }
  showSampleModal.value = false
  message.success(t('detail.sampleSaved'))
  await loadSamples()
  pickSample(res.data.id)
}

/** 保存正文修改到选中示例。 */
async function persistSample(): Promise<void> {
  const cur = samples.value.find((x) => x.id === sampleId.value)
  if (!cur) return
  sampleSaving.value = true
  try {
    const res = await saveApiSample({
      id: cur.id, api_id: apiId.value, name: cur.name, sample_text: sampleText.value,
      status: cur.status, content_type: cur.content_type, seq: cur.seq,
    })
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('common.saveFailed'))
      return
    }
    message.success(t('detail.sampleSaved'))
    await loadSamples()
  } finally {
    sampleSaving.value = false
  }
}

function removeSample(): void {
  const cur = samples.value.find((x) => x.id === sampleId.value)
  if (!cur) return
  dialog.warning({
    title: t('detail.sampleDelete'),
    content: t('detail.sampleDeleteConfirm', { name: cur.name }),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = await deleteApiSample(cur.id)
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.deleteFailed'))
        return
      }
      message.success(t('detail.sampleDeleted'))
      await loadSamples()
    },
  })
}


async function copyText(text: string, label: string): Promise<void> {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    message.success(t('detail.copied', { label }))
  } catch {
    message.warning(text)
  }
}

// ---- 请求代码（cURL / fetch / axios / Go / Python；变量可选替换为当前环境实际值）----
/** 下拉项 key 形如 `lang:resolve`，如 `curl:0` 表示保留占位符的 cURL。 */
const codeOptions = computed<DropdownOption[]>(() => [
  { label: t('detail.curlKeep'), key: 'curl:0' },
  { label: t('detail.curlResolve'), key: 'curl:1' },
  { type: 'divider', key: 'code-divider' },
  { label: 'JavaScript fetch', key: 'fetch:1' },
  { label: 'TypeScript axios', key: 'axios:1' },
  { label: 'Go (net/http)', key: 'go:1' },
  { label: 'Python (requests)', key: 'python:1' },
  { type: 'divider', key: 'mock-divider' },
  { label: t('detail.mockCopy'), key: 'mock:0' },
])

/** Mock 地址：`/mock/<项目ID>/<接口路径>`（免登录，返回该接口的响应样例，变量按通用值渲染）。 */
function mockURL(): string {
  let path = form.url.trim()
  if (path.includes('://')) path = path.slice(path.indexOf('://') + 3).replace(/^[^/]*/, '')
  path = path.split('?')[0].split('#')[0]
  if (!path.startsWith('/')) path = '/' + path
  return `${location.origin}/mock/${projectId.value}${path}`
}

const codeShow = ref(false)
const codeLoading = ref(false)
const codeTitle = ref('')
const codeText = ref('')

const codeLangLabel: Record<string, string> = {
  curl: 'cURL', fetch: 'JavaScript fetch', axios: 'TypeScript axios', go: 'Go', python: 'Python',
}

/** 生成并预览代码片段；resolve=1 时由服务端用当前环境变量值替换占位符（敏感值不回传单值）。 */
async function showCode(key: string): Promise<void> {
  const [lang, resolveFlag] = key.split(':')
  if (!lang) return
  // Mock 地址只复制、不弹窗。
  if (lang === 'mock') {
    await copyText(mockURL(), t('detail.mockLabel'))
    return
  }
  const envId = variableStore.currentEnv[projectId.value] ?? 0
  codeTitle.value = codeLangLabel[lang] ?? lang
  codeText.value = ''
  codeShow.value = true
  codeLoading.value = true
  try {
    const res = await getCode(apiId.value, lang, envId, resolveFlag === '1')
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('detail.codeFailed'))
      codeShow.value = false
      return
    }
    codeText.value = res.data.code
  } finally {
    codeLoading.value = false
  }
}

async function copyCode(): Promise<void> {
    await copyText(codeText.value, codeTitle.value)
}

// 历史版本回滚成功后刷新详情与左侧树。
function onRevisionRestored(): void {
    void load()
    emit('changed')
}

// ---- 导出接口文档（Markdown / 自包含 HTML）：交付与归档用，取「编辑态当前内容」 ----
const exportOptions = computed<DropdownOption[]>(() => [
    { label: t('detail.exportMd'), key: 'md' },
    { label: t('detail.exportHtml'), key: 'html' },
])

/** 组装导出数据：以编辑态表单为准（含未保存改动），便于定稿后立刻导出。 */
function currentExportInput(): ApiExportInput {
    return {
        id: apiId.value,
        api_name: form.api_name,
        group_id: groupId.value,
        protocol_type: form.protocol_type,
        http_method_type: form.http_method_type,
        url: form.url,
        description: form.description,
        http_request_header: JSON.stringify(paramsToHeaders(headerRows.value)),
        http_request_params: JSON.stringify(reqParams.value),
        http_return_type: form.http_return_type,
        http_return_params: JSON.stringify(resParams.value),
        http_return_sample: sample.value,
        develop_language: form.develop_language,
        update_time: loadedUpdateTime.value,
        status: form.status,
        version: form.version,
    }
}

function exportApi(key: string): void {
    const data = currentExportInput()
    const base = safeName(`${form.http_method_type}-${form.api_name}`, `api-${apiId.value}`)
    if (key === 'md') {
        downloadText(`${base}.md`, apiToMarkdown(data, groups.value), 'text/markdown')
        message.success(t('detail.exportMdDone'))
        return
    }
    downloadText(`${base}.html`, apiToStandaloneHtml(data, groups.value), 'text/html')
    message.success(t('detail.exportHtmlDone'))
}
</script>

<template>
  <n-spin :show="loading" class="req-spin">
    <div class="req">
      <header class="req-hd">
        <n-input
          v-if="editing"
          v-model:value="form.api_name"
          size="small"
          class="req-name-input"
          :placeholder="t('api.namePlaceholder')"
        />
        <h1 v-else class="req-name">{{ form.api_name || t('tree.unnamedApi') }}</h1>
        <n-select
          v-if="editing"
          v-model:value="groupId"
          :options="groupOptions"
          size="small"
          class="req-group-select"
          :placeholder="t('api.group')"
        />
        <!-- 查看态：请求协议 → 所属分组，依次跟在接口名称后 -->
        <template v-else>
          <span v-if="form.protocol_type" class="req-meta">{{ form.protocol_type }}</span>
          <n-tag size="small" :type="statusTagType" round class="req-status" data-testid="detail.status">
            {{ statusLabel }}
          </n-tag>
          <span v-if="form.version" class="req-meta">{{ form.version }}</span>
          <span class="req-meta">{{ groupName }}</span>
        </template>
        <template v-if="editing">
          <n-select
            v-model:value="form.status"
            :options="statusOptions"
            size="small"
            class="req-status-select"
            data-testid="detail.statusEdit"
          />
          <n-input
            v-model:value="form.version"
            size="small"
            class="req-version-input"
            :placeholder="t('api.versionPlaceholder')"
            data-testid="detail.version"
          />
        </template>

        <div class="req-ops">
          <env-picker :project-id="projectId" />
          <n-button size="small" quaternary :title="t('tryIt.openTitle')" data-testid="detail.tryIt" @click="tryItShow = true">
            <template #icon><n-icon :component="PlayOutline" /></template>
            {{ t('tryIt.open') }}
          </n-button>
          <n-button size="small" quaternary :title="t('history.title')" data-testid="detail.history" @click="historyShow = true">
            <template #icon><n-icon :component="TimeOutline" /></template>
            {{ t('history.btn') }}
          </n-button>
          <n-dropdown trigger="click" :options="codeOptions" @select="showCode">
            <n-button size="small" quaternary :title="t('detail.codeBtnTitle')">
              <template #icon><n-icon :component="CodeSlashOutline" /></template>
              {{ t('detail.codeBtn') }}
            </n-button>
          </n-dropdown>
          <n-dropdown trigger="click" :options="exportOptions" @select="exportApi">
            <n-button size="small" quaternary :title="t('detail.exportBtnTitle')">
              <template #icon><n-icon :component="DownloadOutline" /></template>
              {{ t('detail.exportBtn') }}
            </n-button>
          </n-dropdown>
          <n-button size="small" quaternary :title="t('detail.refreshTitle')" :loading="loading" @click="refreshApi">
            <template #icon><n-icon :component="RefreshOutline" /></template>
            {{ t('detail.refreshBtn') }}
          </n-button>
        </div>
      </header>

      <div class="req-url">
        <div class="url-box" :class="{ 'url-box--edit': editing }">
          <n-select
            v-if="editing"
            v-model:value="form.http_method_type"
            :options="methodOptions"
            size="small"
            :bordered="false"
            class="url-method"
          />
          <method-tag v-else :method="form.http_method_type" class="url-method--static" />
          <n-input
            v-if="editing"
            v-model:value="form.url"
            size="small"
            :bordered="false"
            class="url-input"
            placeholder="/api/xxx"
          />
          <!-- 查看态：{{var}} 就地高亮，悬停看来源与取值 -->
          <span v-else class="url-static url-input">
            <var-text v-if="form.url" :text="form.url" :project-id="projectId" :api-id="apiId" />
            <span v-else class="url-ph">/api/xxx</span>
          </span>
        </div>
        <n-button v-if="form.url" size="small" quaternary :title="t('detail.copyUrlTitle')" @click="copyText(form.url, 'URL')">
          <template #icon><n-icon :component="CopyOutline" /></template>
        </n-button>

        <template v-if="editing">
          <n-button size="small" @click="cancelEdit">{{ t('common.cancel') }}</n-button>
          <n-button size="small" type="primary" :loading="saving" @click="save">{{ t('common.save') }}</n-button>
        </template>
        <n-button v-else-if="canUpdate" size="small" type="primary" @click="startEdit">
          <template #icon><n-icon :component="CreateOutline" /></template>
          {{ t('common.edit') }}
        </n-button>
      </div>

      <!-- 悬浮大纲：收起为小格子，悬停展开，点击跳段 -->
      <div v-if="sections.length > 1" class="req-ol">
        <div class="ol-cells">
          <button
            v-for="s in sections"
            :key="s.key"
            class="ol-cell"
            :class="{ 'ol-cell--active': activeSec === s.key }"
            :title="s.label"
            @click="scrollToSection(s.key)"
          />
        </div>
        <ul class="ol-list">
          <li
            v-for="s in sections"
            :key="s.key"
            :class="{ 'ol-item--active': activeSec === s.key }"
            @click="scrollToSection(s.key)"
          >
            {{ s.label }}
          </li>
        </ul>
      </div>

      <!-- 垂直排列的各段内容 -->
      <div ref="bodyRef" class="req-body" @scroll="onBodyScroll">
        <section class="req-sec" data-sec="headers">
          <api-param-table v-model="headerRows" kind="header" :editable="editing" :project-id="projectId" :api-id="apiId"
            @edit="editing = true" />
        </section>

        <section class="req-sec" data-sec="params">
          <api-param-table v-model="reqParams" kind="request" :editable="editing" :project-id="projectId" :api-id="apiId"
            @edit="editing = true" />
        </section>

        <section v-if="hasBody || editing" class="req-sec" data-sec="body">
          <div class="sec-hd">
            <span class="sec-title">{{ t('detail.secBody') }}</span>
            <span v-if="!editing && ext.http_request_body_type" class="meta"> · {{ ext.http_request_body_type }}</span>
          </div>
          <template v-if="!editing">
            <dl v-if="bodyFormRows.length" class="kv">
              <div v-for="(r, i) in bodyFormRows" :key="i" class="kv-row">
                <dt>{{ r.name }}<span v-if="r.type && r.type !== 'text'" class="meta">（{{ r.type }}）</span></dt>
                <dd>{{ r.value }}<span v-if="r.description" class="meta"> — {{ r.description }}</span></dd>
              </div>
            </dl>
            <json-block v-else :raw="ext.http_request_body" :empty="t('detail.bodyEmpty')" />
            <template v-if="bodyDescRows.length">
              <p class="sec-sub meta">{{ t('detail.bodyDescsTitle') }}</p>
              <dl class="kv">
                <div v-for="(d, i) in bodyDescRows" :key="'d' + i" class="kv-row"><dt>{{ d.path }}</dt><dd>{{ d.note }}</dd></div>
              </dl>
            </template>
          </template>
          <template v-else>
            <div class="ext-row">
              <span class="ext-lb">{{ t('detail.bodyType') }}</span>
              <n-select v-model:value="extEdit.bodyType" size="small" class="ext-w" :options="bodyTypeOptions" data-testid="ext-body-type" />
            </div>
            <n-input
              v-if="extEdit.bodyType === 'json' || extEdit.bodyType === 'text'"
              v-model:value="extEdit.bodyRaw"
              type="textarea"
              :autosize="{ minRows: 3, maxRows: 14 }"
              class="ext-raw"
              data-testid="ext-body-raw"
              placeholder=""
            />
            <template v-else-if="extEdit.bodyType === 'form' || extEdit.bodyType === 'multipart'">
              <div v-for="(r, i) in extEdit.bodyForm" :key="'bf' + i" class="ext-row">
                <n-input v-model:value="r.name" size="small" :placeholder="t('varForm.name')" data-testid="ext-form-name" />
                <n-input v-model:value="r.value" size="small" :placeholder="t('varForm.value')" />
                <n-input v-model:value="r.type" size="small" class="ext-type" :placeholder="t('detail.colType')" />
                <n-button size="tiny" quaternary type="error" data-testid="ext-form-del" @click="extEdit.bodyForm.splice(i, 1)">
                  {{ t('common.delete') }}
                </n-button>
              </div>
              <n-button size="tiny" data-testid="ext-form-add" @click="extEdit.bodyForm.push({ name: '', value: '', enabled: true, type: 'text' })">
                {{ t('detail.addRow') }}
              </n-button>
            </template>
            <p class="sec-sub meta ext-desc-hd">
              {{ t('detail.bodyDescsTitle') }}
              <n-button size="tiny" data-testid="ext-desc-add" @click="extEdit.bodyDescs.push({ path: '', note: '' })">
                {{ t('detail.addRow') }}
              </n-button>
            </p>
            <div v-for="(d, i) in extEdit.bodyDescs" :key="'bd' + i" class="ext-row">
              <n-input v-model:value="d.path" size="small" class="ext-path" :placeholder="t('detail.descPath')" data-testid="ext-desc-path" />
              <n-input v-model:value="d.note" size="small" :placeholder="t('detail.descNote')" />
              <n-button size="tiny" quaternary type="error" data-testid="ext-desc-del" @click="extEdit.bodyDescs.splice(i, 1)">
                {{ t('common.delete') }}
              </n-button>
            </div>
          </template>
        </section>

        <section v-if="hasAuth || editing" class="req-sec" data-sec="auth">
          <div class="sec-hd"><span class="sec-title">{{ t('detail.secAuth') }}</span></div>
          <template v-if="!editing">
            <dl class="kv">
              <div class="kv-row"><dt>{{ t('detail.authType') }}</dt><dd>{{ str(authObj, 'type') }}</dd></div>
              <div v-if="str(authObj, 'username')" class="kv-row"><dt>{{ t('detail.authUsername') }}</dt><dd>{{ str(authObj, 'username') }}</dd></div>
              <div v-if="str(authObj, 'password')" class="kv-row"><dt>{{ t('detail.authPassword') }}</dt><dd>••••••</dd></div>
              <div v-if="str(authObj, 'token')" class="kv-row"><dt>{{ t('detail.authToken') }}</dt><dd>••••••</dd></div>
              <div v-if="str(authObj, 'key')" class="kv-row"><dt>{{ t('detail.authKey') }}</dt><dd>{{ str(authObj, 'key') }}</dd></div>
              <div v-if="str(authObj, 'value')" class="kv-row"><dt>{{ t('detail.authValue') }}</dt><dd>{{ str(authObj, 'value') }}</dd></div>
              <div v-if="str(authObj, 'in')" class="kv-row"><dt>{{ t('detail.authIn') }}</dt><dd>{{ str(authObj, 'in') }}</dd></div>
            </dl>
          </template>
          <template v-else>
            <div class="ext-row">
              <span class="ext-lb">{{ t('detail.authType') }}</span>
              <n-select v-model:value="extEdit.authType" size="small" class="ext-w" clearable :options="authTypeOptions" data-testid="ext-auth-type" />
            </div>
            <template v-if="extEdit.authType === 'basic'">
              <div class="ext-row">
                <span class="ext-lb">{{ t('detail.authUsername') }}</span>
                <n-input v-model:value="extEdit.authUser" size="small" class="ext-w" data-testid="ext-auth-user" />
              </div>
              <div class="ext-row">
                <span class="ext-lb">{{ t('detail.authPassword') }}</span>
                <n-input v-model:value="extEdit.authPass" size="small" type="password" show-password-on="click" class="ext-w" data-testid="ext-auth-pass" />
              </div>
            </template>
            <div v-if="extEdit.authType === 'bearer'" class="ext-row">
              <span class="ext-lb">{{ t('detail.authToken') }}</span>
              <n-input v-model:value="extEdit.authToken" size="small" type="password" show-password-on="click" class="ext-w" data-testid="ext-auth-token" />
            </div>
            <template v-if="extEdit.authType === 'apikey'">
              <div class="ext-row">
                <span class="ext-lb">{{ t('detail.authKey') }}</span>
                <n-input v-model:value="extEdit.authKey" size="small" class="ext-w" data-testid="ext-auth-key" />
              </div>
              <div class="ext-row">
                <span class="ext-lb">{{ t('detail.authValue') }}</span>
                <n-input v-model:value="extEdit.authValue" size="small" type="password" show-password-on="click" class="ext-w" data-testid="ext-auth-value" />
              </div>
              <div class="ext-row">
                <span class="ext-lb">{{ t('detail.authIn') }}</span>
                <n-select v-model:value="extEdit.authIn" size="small" class="ext-w" :options="authInOptions" />
              </div>
            </template>
          </template>
        </section>

        <section class="req-sec" data-sec="result">
          <api-param-table
            v-model="resParams"
            kind="response"
            :editable="editing"
            :json-source="sample"
            :title-extra="resultTitleExtra"
            :project-id="projectId"
            :api-id="apiId"
            @json-applied="onSampleJson"
            @edit="editing = true"
          />
          <div v-if="editing" class="ext-row">
            <span class="ext-lb">{{ t('detail.retContentType') }}</span>
            <n-select
              v-model:value="extEdit.retContentType"
              size="small"
              class="ext-w"
              filterable
              tag
              clearable
              :options="retContentTypeOptions"
              data-testid="ext-ret-ct"
              @update:value="(v: string | null) => (extEdit.retContentType = v ?? '')"
            />
          </div>
        </section>

        <section class="req-sec" data-sec="sample">
          <div class="sec-hd"><span class="sec-title">{{ t('detail.secSample') }}</span></div>
          <p class="sec-sub meta">{{ t('detail.sampleOk') }}</p>
          <json-block :raw="sample" :empty="t('detail.sampleEmpty')" />
          <p class="sec-sub meta">{{ t('detail.sampleFail') }}</p>
          <json-block :raw="failedSample" empty="" />
          <p class="meta">{{ t('detail.sampleNote') }}</p>

          <!-- 多响应示例：与客户端「保存的响应」互通 -->
          <div v-if="samples.length || canUpdate" class="samples-bar">
            <span class="sec-sub meta">{{ t('detail.samplesTitle') }}</span>
            <n-select
              v-if="samples.length"
              size="small"
              class="samples-select"
              :value="sampleId"
              :options="sampleOptions"
              :placeholder="t('detail.samplePick')"
              @update:value="pickSample"
            />
            <template v-if="canUpdate">
              <n-button size="small" @click="sampleName = ''; showSampleModal = true">{{ t('detail.sampleAdd') }}</n-button>
              <n-button v-if="sampleId" size="small" :loading="sampleSaving" @click="persistSample">{{ t('common.save') }}</n-button>
              <n-button v-if="sampleId" size="small" quaternary type="error" @click="removeSample">{{ t('detail.sampleDelete') }}</n-button>
            </template>
          </div>
          <n-input
            v-if="canUpdate && sampleId"
            v-model:value="sampleText"
            type="textarea"
            class="samples-editor"
            :autosize="{ minRows: 4, maxRows: 18 }"
            placeholder=""
          />
          <json-block v-else-if="sampleId" :raw="sampleText" empty="" />
          <n-modal
            v-model:show="showSampleModal"
            preset="dialog"
            :title="t('detail.sampleAdd')"
            :positive-text="t('common.confirm')"
            :negative-text="t('common.cancel')"
            :loading="sampleSaving"
            style="width: 420px"
            @positive-click="createSample"
          >
            <n-input v-model:value="sampleName" :placeholder="t('detail.sampleNamePh')" @keyup.enter="createSample" />
            <p class="meta">{{ t('detail.sampleAddTip') }}</p>
          </n-modal>
        </section>

        <section v-if="hasSettings || editing" class="req-sec" data-sec="settings">
          <div class="sec-hd"><span class="sec-title">{{ t('detail.secSettings') }}</span></div>
          <dl v-if="!editing" class="kv">
            <div v-if="settingsObj.timeout" class="kv-row"><dt>{{ t('detail.setTimeout') }}</dt><dd>{{ settingsObj.timeout }}s</dd></div>
            <div class="kv-row"><dt>{{ t('detail.setFollowRedirects') }}</dt><dd>{{ triLabel(settingsObj.followRedirects) }}</dd></div>
            <div v-if="settingsObj.maxRedirects" class="kv-row"><dt>{{ t('detail.setMaxRedirects') }}</dt><dd>{{ settingsObj.maxRedirects }}</dd></div>
            <div class="kv-row"><dt>{{ t('detail.setInsecureSsl') }}</dt><dd>{{ triLabel(settingsObj.insecureSsl) }}</dd></div>
            <div v-if="settingsObj.encodeUrl != null" class="kv-row"><dt>{{ t('detail.setEncodeUrl') }}</dt><dd>{{ settingsObj.encodeUrl ? t('detail.triOn') : t('detail.triOff') }}</dd></div>
          </dl>
          <template v-else>
            <div class="ext-row">
              <span class="ext-lb">{{ t('detail.setTimeout') }}</span>
              <n-input-number v-model:value="extEdit.timeout" size="small" class="ext-num" :min="0" :show-button="false" data-testid="ext-timeout" />
              <span class="meta">s</span>
              <span class="ext-lb">{{ t('detail.setMaxRedirects') }}</span>
              <n-input-number v-model:value="extEdit.maxRedirects" size="small" class="ext-num" :min="0" :show-button="false" data-testid="ext-maxred" />
            </div>
            <div class="ext-row">
              <span class="ext-lb">{{ t('detail.setFollowRedirects') }}</span>
              <n-select v-model:value="extEdit.followRedirects" size="small" class="ext-w" :options="triStateOptions" data-testid="ext-follow" />
              <span class="ext-lb">{{ t('detail.setInsecureSsl') }}</span>
              <n-select v-model:value="extEdit.insecureSsl" size="small" class="ext-w" :options="triStateOptions" data-testid="ext-insecure" />
            </div>
            <div class="ext-row">
              <span class="ext-lb">{{ t('detail.setEncodeUrl') }}</span>
              <n-select v-model:value="extEdit.encodeUrl" size="small" class="ext-w" :options="encodeUrlOptions" data-testid="ext-encode" />
            </div>
          </template>
        </section>

        <section v-if="!editing && hasScript" class="req-sec" data-sec="script">
          <div class="sec-hd"><span class="sec-title">{{ t('detail.secScript') }}</span></div>
          <template v-if="str(scriptObj, 'preRequest')">
            <p class="sec-sub meta">{{ t('detail.scriptPre') }}</p>
            <json-block :raw="str(scriptObj, 'preRequest')" empty="" />
          </template>
          <template v-if="str(scriptObj, 'postResponse')">
            <p class="sec-sub meta">{{ t('detail.scriptPost') }}</p>
            <json-block :raw="str(scriptObj, 'postResponse')" empty="" />
          </template>
          <dl v-if="varsRows.length" class="kv">
            <div v-for="(v, i) in varsRows" :key="'v' + i" class="kv-row"><dt>{{ v.name }}</dt><dd>{{ v.value }}</dd></div>
          </dl>
          <dl v-if="assertRows.length" class="kv">
            <div v-for="(a, i) in assertRows" :key="'a' + i" class="kv-row"><dt>{{ a.name }}</dt><dd>{{ a.expr }}</dd></div>
          </dl>
        </section>

        <section v-if="!editing && form.develop_language" class="req-sec" data-sec="info">
          <div class="sec-hd"><span class="sec-title">{{ t('detail.secInfo') }}</span></div>
          <dl class="kv">
            <div class="kv-row"><dt>{{ t('api.devLang') }}</dt><dd>{{ form.develop_language }}</dd></div>
          </dl>
        </section>

        <!-- 文档：该接口的 Markdown 文档（编辑态可写，查看态渲染） -->
        <section class="req-sec" data-sec="doc">
          <div class="sec-hd"><span class="sec-title">{{ t('detail.secDoc') }}</span></div>
          <md-editor
            v-if="editing"
            v-model="form.description"
            preview-theme="default"
            class="md-edit"
            @on-upload-img="onUploadImg"
          />
          <md-preview
            v-else-if="form.description"
            :model-value="form.description"
            preview-theme="default"
            class="md-doc"
          />
          <p v-else class="meta">{{ t('detail.docEmpty') }}</p>
        </section>
      </div>

      <!-- 长页一键返回顶部 -->
      <button v-if="showTop" class="req-top" :title="t('detail.backToTop')" @click="scrollToTop">
        <n-icon :component="ArrowUpOutline" />
      </button>
    </div>

    <!-- 在线调试（RM-01）：服务端代发请求，复用当前环境变量 -->
    <try-it-panel
      v-model:show="tryItShow"
      :project-id="projectId"
      :api-id="apiId"
      :method="form.http_method_type"
      :url="form.url"
      :headers="headerRows"
      :params="reqParams"
      :body-type="debugBodyType"
      :body="debugBodyRaw"
      :form="debugBodyForm"
      :auth="debugAuth"
    />

    <!-- 历史版本（RM-05）：列表 + 字段级 diff + 回滚 -->
    <revision-history v-model:show="historyShow" type="api" :id="apiId" @restored="onRevisionRestored" />

    <!-- 请求代码预览（cURL / fetch / axios / Go / Python） -->
    <n-modal v-model:show="codeShow" preset="card" :title="t('detail.codeModalTitle', { title: codeTitle })" style="width: 680px">
      <n-spin :show="codeLoading">
        <pre class="snippet">{{ codeText }}</pre>
      </n-spin>
      <template #footer>
        <div class="code-foot">
          <span class="meta">{{ t('detail.codeTip') }}</span>
          <div class="row-actions">
            <n-button size="small" @click="codeShow = false">{{ t('common.close') }}</n-button>
            <n-button size="small" type="primary" :disabled="!codeText" @click="copyCode">{{ t('common.copy') }}</n-button>
          </div>
        </div>
      </template>
    </n-modal>
  </n-spin>
</template>

<style scoped>
.req-spin {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
}

/* naive-ui 的 spin 根节点即传入类名所在元素，内容层需显式撑满。 */
.req-spin :deep(.n-spin-content) {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.req {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #fff;
}

/* ---- 名称栏 ---- */
.req-hd {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 20px 0;
}

.req-name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  line-height: 24px;
}

.req-name-input {
  max-width: 320px;
}

.req-group-select {
  width: 180px;
  flex: 0 0 auto;
}

.req-status {
  flex: 0 0 auto;
}

.req-status-select {
  width: 108px;
  flex: 0 0 auto;
}

.req-version-input {
  width: 110px;
  flex: 0 0 auto;
}

/* 查看态的协议 / 分组小胶囊 */
.req-meta {
  flex: 0 0 auto;
  padding: 0 8px;
  border-radius: 999px;
  background: #f5f6f8;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 20px;
}

.req-ops {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
}

/* ---- 多响应示例 ---- */
.samples-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}
.samples-bar .sec-sub {
  margin: 0;
}
.samples-select {
  width: 220px;
}
.samples-editor {
  margin-top: 8px;
  font-family: var(--font-mono, monospace);
}

/* ---- URL 栏（Postman 请求行） ---- */
.req-url {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 20px;
  border-bottom: 1px solid var(--app-border);
}

.url-box {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  height: 32px;
  padding: 0 4px;
  border: 1px solid var(--app-border);
  border-radius: 4px;
  background: #fff;
}

.url-box--edit:focus-within {
  border-color: var(--app-accent);
}

.url-box :deep(.n-input),
.url-box :deep(.n-input-wrapper),
.url-box :deep(.n-input__border),
.url-box :deep(.n-input__state-border) {
  background: transparent;
}

.url-method {
  flex: 0 0 96px;
  width: 96px;
}

.url-method--static {
  flex: 0 0 auto;
  margin: 0 4px 0 8px;
}

.url-input {
  flex: 1 1 auto;
  min-width: 0;
}

.url-input :deep(input) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
}

/* 查看态 URL：与编辑态输入框同样的字体与内边距，仅去掉输入框外观 */
.url-static {
  display: block;
  padding: 0 7px;
  overflow: hidden;
  color: var(--app-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
  line-height: 30px;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.url-ph {
  color: #c2c8ce;
}

/* ---- 悬浮大纲 ---- */
.req-ol {
  position: absolute;
  top: 108px;
  right: 14px;
  z-index: 6;
  padding: 5px;
  border: 1px solid transparent;
  border-radius: 6px;
}

.req-ol:hover {
  border-color: var(--app-border);
  background: #fff;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.08);
}

.ol-cells {
  display: grid;
  grid-template-columns: repeat(2, 12px);
  gap: 4px;
}

.req-ol:hover .ol-cells {
  display: none;
}

.ol-cell {
  width: 12px;
  height: 12px;
  padding: 0;
  border: 0;
  border-radius: 3px;
  background: #d8dce1;
  cursor: pointer;
}

.ol-cell:hover {
  background: #b9bfc6;
}

.ol-cell--active {
  background: var(--app-accent);
}

.ol-list {
  display: none;
  list-style: none;
  margin: 0;
  padding: 0;
  min-width: 104px;
}

.req-ol:hover .ol-list {
  display: block;
}

.ol-list li {
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--app-text);
  cursor: pointer;
  white-space: nowrap;
}

.ol-list li:hover {
  background: #f2f3f5;
  color: var(--app-accent);
}

.ol-item--active {
  color: var(--app-accent);
  font-weight: 600;
}

/* ---- 垂直排列的各段 ---- */
.req-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 16px 20px 24px;
}

.req-sec {
  display: flex;
  flex-direction: column;
  gap: 8px;
  /* 阅读限宽：1440 宽窗口下正文行宽接近 1100px，长 URL / 长描述读起来很累（UI-11）。
     表格类（参数表）在 960 内仍够用，窄屏自动退化。 */
  max-width: 960px;
}

/* 段间分隔：除首段外都加一条上分隔线 */
.req-sec + .req-sec {
  margin-top: 18px;
  padding-top: 18px;
  border-top: 1px solid var(--app-border);
}

.sec-hd {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sec-title {
  font-size: 13px;
  font-weight: 600;
}

.sec-sub {
  margin: 0;
  font-size: 12px;
}

/* Markdown 文档：编辑器定高、预览去掉外层留白 */
.md-edit {
  width: 100%;
  height: 340px;
}

.md-doc :deep(.md-editor-preview-wrapper) {
  padding: 0;
}

.md-doc :deep(.md-editor-preview) {
  padding: 0;
  background: transparent;
  font-size: 13px;
  line-height: 1.7;
}

.kv {
  margin: 0;
  border: 1px solid var(--app-border);
  border-radius: 4px;
  overflow: hidden;
}

.kv-row {
  display: flex;
  align-items: baseline;
  gap: 16px;
  padding: 9px 14px;
  font-size: 13px;
}

.kv-row dt {
  flex: 0 0 84px;
  color: var(--app-muted);
  font-size: 12px;
}

.kv-row dd {
  margin: 0;
  min-width: 0;
  word-break: break-all;
}

/* ---- 返回顶部 ---- */
.req-top {
  position: absolute;
  right: 16px;
  bottom: 24px;
  z-index: 6;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 1px solid var(--app-border);
  border-radius: 50%;
  background: #fff;
  color: var(--app-muted);
  font-size: 18px;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.req-top:hover {
  border-color: var(--app-accent);
  color: var(--app-accent);
}

/* ---- 请求代码预览（类名避开 JsonBlock 的 .code-block） ---- */
.snippet {
  max-height: 420px;
  margin: 0;
  padding: 12px 14px;
  overflow: auto;
  border: 1px solid var(--app-border);
  border-radius: 4px;
  background: #fafbfc;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.code-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

/* ---- 移动端：URL 栏与操作区换行、隐藏悬浮大纲（避免压住内容） ---- */
@media (max-width: 768px) {
  .req-ol {
    display: none;
  }

  .req-hd,
  .req-url {
    flex-wrap: wrap;
    padding: 10px 12px;
  }

  .req-ops {
    width: 100%;
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .url-box {
    flex: 1 1 100%;
  }

  .req-name {
    font-size: 16px;
  }

  .req-body {
    padding: 12px 12px 40px;
  }

  .code-foot {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (max-width: 900px) {
  .req-hd {
    flex-wrap: wrap;
    padding: 12px 14px 0;
  }
  .req-url,
  .req-body {
    padding-left: 14px;
    padding-right: 14px;
  }
}

/* ---- 富字段编辑行（⑤）：标签 + 控件的紧凑横排 ---- */
.ext-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 6px 0;
}

.ext-lb {
  flex: none;
  min-width: 92px;
  color: var(--text-2, #666);
  font-size: 13px;
}

.ext-w { width: 230px; flex: none; }
.ext-num { width: 110px; flex: none; }
.ext-type { width: 110px; flex: none; }
.ext-path { width: 230px; flex: none; }
.ext-raw { margin: 6px 0; }
.ext-desc-hd { display: flex; align-items: center; gap: 10px; margin-top: 12px; }

@media (max-width: 768px) {
  .ext-row { flex-wrap: wrap; }
  .ext-w, .ext-path { width: 100%; }
}
</style>
