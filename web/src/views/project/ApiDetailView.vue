<script setup lang="ts">
// 接口详情（Postman 请求编辑器样式）：顶部名称栏 + URL 栏，下方 参数/头部/返回/样例/说明 页签。
// 查看态与编辑态共用同一版式——编辑态仅把只读文本换成输入控件。
// 参数数组以 JSON 存入后端文本列；fieldName/type/description 键与字段映射回填约定一致。
import { deleteApi, getApi, getCurl, listGroups, updateApi } from '@/api/content'
import ApiParamTable from '@/components/ApiParamTable.vue'
import JsonBlock from '@/components/JsonBlock.vue'
import MethodTag from '@/components/MethodTag.vue'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { ApiHeader, ApiParam, Group } from '@/types/entities'
import { headersToParams, paramsToHeaders } from '@/utils/api-params'
import { CloseOutline, CodeSlashOutline, CopyOutline, CreateOutline, TrashOutline } from '@vicons/ionicons5'
import {
    NButton, NForm, NFormItem, NIcon, NInput, NSelect, NSpin, NTabPane, NTabs, useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()

/** embedded：由列表页（?api=<id>）内嵌渲染，关闭键改为 emit('close')；apiId 优先取 props。 */
const props = defineProps<{ apiId?: number; embedded?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'changed'): void }>()

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
const headers = ref<ApiHeader[]>([])
const reqParams = ref<ApiParam[]>([])
const resParams = ref<ApiParam[]>([])
const sample = ref('')
const sampleTab = ref('ok')

/** 请求头按参数行形态编辑，写入时还原为 ApiHeader。 */
const headerRows = computed<ApiParam[]>({
  get: () => headersToParams(headers.value),
  set: (rows) => {
    headers.value = paramsToHeaders(rows)
  },
})

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].map((m) => ({ label: m, value: m }))
const groupOptions = computed(() => [
  { label: '未分组', value: 0 },
  ...groups.value.map((g) => ({ label: g.title, value: g.id })),
])
const groupName = computed(() => groups.value.find((g) => g.id === groupId.value)?.title || '未分组')

const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))

/** 平台统一失败响应示例（与 doc/api_docs/common/error-codes.yml 约定一致）。 */
const failedSample = JSON.stringify({ code: 22, msg: '参数错误', data: null })

/** 页签：参数/头部/返回带上已填写条数，样例与说明为纯信息页。 */
const tabs = computed(() => [
  { key: 'params', label: '请求参数', count: reqParams.value.filter((p) => p.fieldName.trim()).length },
  { key: 'headers', label: '请求头部', count: headers.value.filter((h) => h.name.trim()).length },
  { key: 'result', label: '返回参数', count: resParams.value.filter((p) => p.fieldName.trim()).length },
  { key: 'sample', label: '响应样例', count: 0 },
  { key: 'info', label: '说明', count: 0 },
])
const tab = ref('params')

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

async function load(): Promise<void> {
  if (!apiId.value) return
  loading.value = true
  try {
    const res = await getApi(apiId.value)
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || '接口加载失败')
      void back()
      return
    }
    const a = res.data
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
    headers.value = parseArray<ApiHeader>(a.http_request_header)
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

watch(apiId, () => {
  tab.value = 'params'
  sampleTab.value = 'ok'
  void load()
})
onMounted(() => {
  void loadGroups()
  void load()
})

function back(): void {
  if (props.embedded) {
    emit('close')
    return
  }
  void router.push({ name: 'projectApis', params: { projectId: projectId.value } })
}

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
  headers: ApiHeader[]
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
    headers: headers.value.map((h) => ({ ...h })),
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
    headers.value = draft.headers
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
      message.error('响应样例需为合法 JSON')
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
      http_request_header: headers.value,
      http_request_params: reqParams.value,
      http_return_params: resParams.value,
      http_return_sample: sampleVal,
    })
    if (res.code !== ErrCode.Success) {
      message.error(res.msg || '保存失败')
      return
    }
    message.success('已保存')
    draft = null
    editing.value = false
    clearEditQuery()
    emit('changed')
    await load()
  } finally {
    saving.value = false
  }
}

/** 响应 JSON 生成字段后，样例框为空则顺带填入，避免两处内容不一致。 */
function onSampleJson(text: string): void {
  if (!sample.value.trim()) sample.value = text.trim()
}

async function copyText(text: string, label: string): Promise<void> {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    message.success(`${label}已复制`)
  } catch {
    message.warning(text)
  }
}

async function copyCurl(): Promise<void> {
  const res = await getCurl(apiId.value)
  if (res.code !== ErrCode.Success || !res.data) {
    message.error(res.msg || 'cURL 生成失败')
    return
  }
  await copyText(res.data.curl, 'cURL')
}

function remove(): void {
  dialog.warning({
    title: '删除接口',
    content: `确认删除接口「${form.api_name}」？删除后可在回收站恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const res = await deleteApi(apiId.value)
      if (res.code !== ErrCode.Success) message.error(res.msg || '删除失败')
      emit('changed')
      back()
    },
  })
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
          placeholder="接口名称"
        />
        <h1 v-else class="req-name">{{ form.api_name || '未命名接口' }}</h1>
        <method-tag :method="form.http_method_type" />
        <span class="req-group meta ellipsis" :title="groupName">{{ groupName }}</span>

        <div class="req-ops">
          <n-button size="small" quaternary title="复制 cURL" @click="copyCurl">
            <template #icon><n-icon :component="CodeSlashOutline" /></template>
            cURL
          </n-button>
          <n-button v-if="canDelete && !editing" size="small" quaternary title="删除接口" @click="remove">
            <template #icon><n-icon :component="TrashOutline" /></template>
          </n-button>
          <n-button v-if="embedded" size="small" quaternary title="关闭详情" @click="back">
            <template #icon><n-icon :component="CloseOutline" /></template>
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
          <method-tag v-else plain :method="form.http_method_type" class="url-method--static" />
          <n-input
            v-model:value="form.url"
            size="small"
            :bordered="false"
            :readonly="!editing"
            class="url-input"
            placeholder="/api/xxx"
          />
        </div>
        <n-button v-if="form.url" size="small" quaternary title="复制 URL" @click="copyText(form.url, 'URL')">
          <template #icon><n-icon :component="CopyOutline" /></template>
        </n-button>

        <template v-if="editing">
          <n-button size="small" @click="cancelEdit">取消</n-button>
          <n-button size="small" type="primary" :loading="saving" @click="save">保存</n-button>
        </template>
        <n-button v-else-if="canUpdate" size="small" type="primary" @click="startEdit">
          <template #icon><n-icon :component="CreateOutline" /></template>
          编辑
        </n-button>
      </div>

      <nav class="req-tabs">
        <button
          v-for="t in tabs"
          :key="t.key"
          class="rt"
          :class="{ 'rt--active': tab === t.key }"
          @click="tab = t.key"
        >
          {{ t.label }}<sup v-if="t.count">{{ t.count }}</sup>
        </button>
      </nav>

      <div class="req-body">
        <api-param-table v-if="tab === 'params'" v-model="reqParams" kind="request" :editable="editing" />
        <api-param-table v-else-if="tab === 'headers'" v-model="headerRows" kind="header" :editable="editing" />
        <api-param-table
          v-else-if="tab === 'result'"
          v-model="resParams"
          kind="response"
          :editable="editing"
          :json-source="sample"
          @json-applied="onSampleJson"
        />

        <div v-else-if="tab === 'sample'" class="sample-pane">
          <n-tabs v-model:value="sampleTab" type="segment" size="small">
            <n-tab-pane name="ok" tab="成功响应">
              <json-block :raw="sample" empty="未配置成功响应样例" />
            </n-tab-pane>
            <n-tab-pane name="fail" tab="通用失败响应">
              <json-block :raw="failedSample" empty="" />
              <p class="meta">平台统一响应结构，非本接口专属示例。</p>
            </n-tab-pane>
          </n-tabs>
        </div>

        <template v-else>
          <n-form v-if="editing" label-placement="left" label-width="80" class="info-form">
            <n-form-item label="所属分组">
              <n-select v-model:value="groupId" :options="groupOptions" />
            </n-form-item>
            <n-form-item label="请求协议"><n-input v-model:value="form.protocol_type" placeholder="HTTP" /></n-form-item>
            <n-form-item label="返回类型"><n-input v-model:value="form.http_return_type" placeholder="JSON" /></n-form-item>
            <n-form-item label="开发语言"><n-input v-model:value="form.develop_language" placeholder="如 go / php" /></n-form-item>
            <n-form-item label="说明">
              <n-input v-model:value="form.description" type="textarea" :rows="3" />
            </n-form-item>
          </n-form>

          <dl v-else class="kv">
            <div class="kv-row"><dt>接口名称</dt><dd>{{ form.api_name || '—' }}</dd></div>
            <div class="kv-row"><dt>所属分组</dt><dd>{{ groupName }}</dd></div>
            <div class="kv-row"><dt>请求方式</dt><dd>{{ form.http_method_type || '—' }}</dd></div>
            <div class="kv-row"><dt>请求协议</dt><dd>{{ form.protocol_type || '—' }}</dd></div>
            <div class="kv-row"><dt>返回类型</dt><dd>{{ form.http_return_type || '—' }}</dd></div>
            <div class="kv-row"><dt>开发语言</dt><dd>{{ form.develop_language || '—' }}</dd></div>
            <div class="kv-row"><dt>URL</dt><dd class="row-mono">{{ form.url || '—' }}</dd></div>
            <div class="kv-row kv-row--block"><dt>说明</dt><dd class="muted">{{ form.description || '—' }}</dd></div>
          </dl>
        </template>
      </div>
    </div>
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

.req-group {
  min-width: 0;
  max-width: 220px;
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
  min-width: 44px;
}

.url-input {
  flex: 1 1 auto;
  min-width: 0;
}

.url-input :deep(input) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
}

/* ---- 页签（Postman 下划线式 + 计数上标） ---- */
.req-tabs {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 20px;
  border-bottom: 1px solid var(--app-border);
}

.rt {
  position: relative;
  height: 38px;
  padding: 0 12px;
  border: 0;
  background: none;
  color: var(--app-text);
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
}

.rt:hover {
  color: var(--app-accent);
}

.rt--active {
  color: var(--app-accent);
  font-weight: 600;
}

.rt--active::after {
  content: '';
  position: absolute;
  left: 8px;
  right: 8px;
  bottom: -1px;
  height: 2px;
  background: var(--app-accent);
}

.rt sup {
  margin-left: 3px;
  color: var(--app-muted);
  font-size: 10px;
  font-weight: 500;
}

.rt--active sup {
  color: var(--app-accent);
}

/* ---- 页签内容 ---- */
.req-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 16px 20px 24px;
}

.sample-pane :deep(.n-tabs-nav) {
  margin-bottom: 8px;
}

.info-form {
  max-width: 640px;
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
  border-bottom: 1px solid var(--app-border);
  font-size: 13px;
}

.kv-row:last-child {
  border-bottom: 0;
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

@media (max-width: 900px) {
  .req-hd {
    flex-wrap: wrap;
    padding: 12px 14px 0;
  }
  .req-url,
  .req-tabs,
  .req-body {
    padding-left: 14px;
    padding-right: 14px;
  }
}
</style>
