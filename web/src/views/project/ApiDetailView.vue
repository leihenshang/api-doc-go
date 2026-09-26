<script setup lang="ts">
// 接口详情（Postman 请求编辑器样式）：名称栏 + URL 栏，下方按「请求头部 / 请求参数 / 返回参数 / 响应样例 / 基本信息(有值时) / 文档」垂直排列。
// 右侧悬浮大纲（收起为小格子、悬停展开、点击跳段）与长页「返回顶部」；文档一节用 MdEditor/MdPreview 承载 Markdown。
// 请求头直接以 ApiParam[] 承载编辑（与 ApiParamTable 同构），只在载入/保存时与 ApiHeader[] 互转，避免中间态丢输入。
import { getApi, getCode, listGroups, updateApi } from '@/api/content'
import ApiParamTable from '@/components/ApiParamTable.vue'
import EnvPicker from '@/components/EnvPicker.vue'
import JsonBlock from '@/components/JsonBlock.vue'
import MethodTag from '@/components/MethodTag.vue'
import VarText from '@/components/VarText.vue'
import {
    apiToMarkdown, apiToStandaloneHtml, downloadText, safeName, type ApiExportInput,
} from '@/utils/export-doc'
import { useMdUpload } from '@/composables/md-upload'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { useVariableStore } from '@/stores/variable'
import { ErrCode } from '@/types/api'
import type { ApiHeader, ApiParam, Group } from '@/types/entities'
import { headersToParams, paramsToHeaders } from '@/utils/api-params'
import { ArrowUpOutline, CodeSlashOutline, CopyOutline, CreateOutline, DownloadOutline } from '@vicons/ionicons5'
import { MdEditor, MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import 'md-editor-v3/lib/style.css'
import {
    NButton, NDropdown, NIcon, NInput, NModal, NSelect, NSpin, useDialog, useMessage,
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
const groups = ref<Group[]>([])

const groupId = ref(0)
const groupIdSecond = ref(0)
const form = reactive({
  api_name: '', http_method_type: 'GET', url: '', protocol_type: 'HTTP',
  description: '', develop_language: '', http_return_type: 'JSON',
})
/** 请求头以参数行形态编辑（表格直接改这份数组），保存时再转回 ApiHeader[]。 */
const headerRows = ref<ApiParam[]>([])
const reqParams = ref<ApiParam[]>([])
const resParams = ref<ApiParam[]>([])
const sample = ref('')

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].map((m) => ({ label: m, value: m }))

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
    { key: 'result', label: t('param.responseTitle') },
    { key: 'sample', label: t('detail.secSample') },
  ]
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

/** 加载失败或详情不可用时退回列表（空态占位）。 */
function goPlaceholder(): void {
  void router.replace({ name: 'projectApis', params: { projectId: projectId.value } })
}

async function load(): Promise<void> {
  if (!apiId.value) return
  loading.value = true
  try {
    const res = await getApi(apiId.value)
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('detail.loadFailed'))
      goPlaceholder()
      return
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
    })
    headerRows.value = headersToParams(parseArray<ApiHeader>(a.http_request_header))
    reqParams.value = parseArray<ApiParam>(a.http_request_params)
    resParams.value = parseArray<ApiParam>(a.http_return_params)
    sample.value = pretty(a.http_return_sample)
  } finally {
    loading.value = false
  }
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
    const res = await updateApi({
      id: apiId.value,
      project_id: projectId.value,
      group_id: groupId.value,
      group_id_second: groupIdSecond.value,
      ...form,
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
          <span class="req-meta">{{ groupName }}</span>
        </template>

        <div class="req-ops">
          <env-picker :project-id="projectId" />
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
          <api-param-table v-model="headerRows" kind="header" :editable="editing" :project-id="projectId" :api-id="apiId" />
        </section>

        <section class="req-sec" data-sec="params">
          <api-param-table v-model="reqParams" kind="request" :editable="editing" :project-id="projectId" :api-id="apiId" />
        </section>

        <section class="req-sec" data-sec="result">
          <api-param-table
            v-model="resParams"
            kind="response"
            :editable="editing"
            :json-source="sample"
            :title-extra="form.http_return_type"
            :project-id="projectId"
            :api-id="apiId"
            @json-applied="onSampleJson"
          />
        </section>

        <section class="req-sec" data-sec="sample">
          <div class="sec-hd"><span class="sec-title">{{ t('detail.secSample') }}</span></div>
          <p class="sec-sub meta">{{ t('detail.sampleOk') }}</p>
          <json-block :raw="sample" :empty="t('detail.sampleEmpty')" />
          <p class="sec-sub meta">{{ t('detail.sampleFail') }}</p>
          <json-block :raw="failedSample" empty="" />
          <p class="meta">{{ t('detail.sampleNote') }}</p>
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
</style>
