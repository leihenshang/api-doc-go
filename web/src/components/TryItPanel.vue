<script setup lang="ts">
// 在线调试面板（RM-01）：把接口详情页当前表单带过来，由服务端代发请求。
// 变量/环境由服务端解析；SSRF 校验与审计也在服务端，前端只负责编辑与展示。
import { sendProxy } from '@/api/content'
import ApiParamTable from '@/components/ApiParamTable.vue'
import EnvPicker from '@/components/EnvPicker.vue'
import { useVariableStore } from '@/stores/variable'
import { ErrCode } from '@/types/api'
import type {
  ApiParam, ProxyAuth, ProxyFormRow, ProxySendResp,
} from '@/types/entities'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  NButton, NCollapse, NCollapseItem, NInput, NModal, NSelect, NSpin, NTabs, NTabPane, useMessage,
} from 'naive-ui'

const props = defineProps<{
  show: boolean
  projectId: number
  apiId: number
  method: string
  url: string
  headers: ApiParam[]
  params: ApiParam[]
  bodyType: string
  body: string
  form: ProxyFormRow[]
  auth: ProxyAuth | null
}>()

const emit = defineEmits<{ (e: 'update:show', value: boolean): void }>()
const { t } = useI18n()
const message = useMessage()
const variableStore = useVariableStore()

const show = computed({
  get: () => props.show,
  set: (v: boolean) => emit('update:show', v),
})

const reqMethod = ref('GET')
const reqURL = ref('')
const headerRows = ref<ApiParam[]>([])
const queryRows = ref<ApiParam[]>([])
const reqBodyType = ref('none')
const bodyText = ref('')
const formRows = ref<ProxyFormRow[]>([])
const authType = ref('')
const authUser = ref('')
const authPass = ref('')
const authToken = ref('')
const authKey = ref('')
const authValue = ref('')
const authIn = ref('header')

const sending = ref(false)
const resp = ref<ProxySendResp | null>(null)
const activeTab = ref('headers')
const multipartHint = ref(false)

const methodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD'].map((m) => ({ label: m, value: m }))
const bodyTypeOptions = computed(() => [
  { label: t('tryIt.bodyNone'), value: 'none' },
  { label: t('tryIt.bodyJson'), value: 'json' },
  { label: t('tryIt.bodyText'), value: 'text' },
  { label: t('tryIt.bodyForm'), value: 'form' },
])
const authTypeOptions = computed(() => [
  { label: t('tryIt.authNone'), value: '' },
  { label: t('tryIt.authBasic'), value: 'basic' },
  { label: t('tryIt.authBearer'), value: 'bearer' },
  { label: t('tryIt.authApiKey'), value: 'apikey' },
])
const authInOptions = computed(() => [
  { label: t('tryIt.inHeader'), value: 'header' },
  { label: t('tryIt.inQuery'), value: 'query' },
])

/** 打开时把接口详情的当前表单复制进面板，避免直接改动详情页状态。 */
function reset(): void {
  reqMethod.value = (props.method || 'GET').toUpperCase()
  reqURL.value = props.url || ''
  headerRows.value = (props.headers || []).filter((r) => r.fieldName.trim()).map((r) => ({ ...r }))
  queryRows.value = (props.params || []).filter((r) => r.fieldName.trim()).map((r) => ({ ...r }))
  const bt = props.bodyType === 'multipart' ? 'form' : (props.bodyType || 'none')
  reqBodyType.value = ['none', 'json', 'text', 'form'].includes(bt) ? bt : 'none'
  bodyText.value = props.body || ''
  formRows.value = (props.form || []).map((r) => ({ ...r, enabled: r.enabled !== false }))
  multipartHint.value = props.bodyType === 'multipart'
  const a = props.auth
  authType.value = a?.type && !['inherit', 'none'].includes(a.type) ? a.type : ''
  authUser.value = a?.username || ''
  authPass.value = a?.password || ''
  authToken.value = a?.token || ''
  authKey.value = a?.key || ''
  authValue.value = a?.value || ''
  authIn.value = a?.in || 'header'
  activeTab.value = 'headers'
  resp.value = null
}

watch(() => props.show, (v) => {
  if (v) reset()
})

const prettyBody = computed(() => {
  const r = resp.value
  if (!r) return ''
  if ((r.content_type || '').toLowerCase().includes('json')) {
    try {
      return JSON.stringify(JSON.parse(r.body), null, 2)
    } catch {
      return r.body
    }
  }
  return r.body
})

const statusClass = computed(() => {
  const s = resp.value?.status ?? 0
  if (s >= 200 && s < 300) return 'tryit-status--ok'
  if (s >= 300 && s < 400) return 'tryit-status--warn'
  return 'tryit-status--err'
})

function fmtSize(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

function activeAuth(): ProxyAuth | null {
  if (!authType.value) return null
  return {
    type: authType.value,
    username: authUser.value,
    password: authPass.value,
    token: authToken.value,
    key: authKey.value,
    value: authValue.value,
    in: authIn.value,
  }
}

async function send(): Promise<void> {
  const u = reqURL.value.trim()
  if (!u) {
    message.warning(t('tryIt.urlRequired'))
    return
  }
  sending.value = true
  try {
    const res = await sendProxy({
      project_id: props.projectId,
      api_id: props.apiId || undefined,
      env_id: variableStore.currentEnv[props.projectId] ?? 0,
      method: reqMethod.value,
      url: u,
      headers: headerRows.value
        .filter((r) => r.fieldName.trim())
        .map((r) => ({ name: r.fieldName, value: r.example || '', enabled: true })),
      query: queryRows.value
        .filter((r) => r.fieldName.trim())
        .map((r) => ({ name: r.fieldName, value: r.example || '', enabled: true })),
      body_type: reqBodyType.value,
      body: bodyText.value,
      form: formRows.value
        .filter((r) => (r.name ?? '').trim())
        .map((r) => ({ name: r.name ?? '', value: r.value ?? '', enabled: r.enabled !== false })),
      auth: activeAuth(),
    })
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('tryIt.sendFailed'))
      return
    }
    resp.value = res.data
  } catch {
    message.error(t('tryIt.sendFailed'))
  } finally {
    sending.value = false
  }
}

async function copyBody(): Promise<void> {
  try {
    await navigator.clipboard.writeText(prettyBody.value)
    message.success(t('tryIt.copied'))
  } catch {
    message.warning(t('tryIt.copyFailed'))
  }
}
</script>

<template>
  <n-modal v-model:show="show" preset="card" :title="t('tryIt.title')" style="width: 860px" data-testid="tryit.modal">
    <div class="tryit-scroll">
    <div class="tryit-head">
      <env-picker :project-id="projectId" data-testid="tryit.env" />
      <n-select v-model:value="reqMethod" :options="methodOptions" size="small" class="tryit-method" data-testid="tryit.method" />
      <n-input v-model:value="reqURL" size="small" class="tryit-url" :placeholder="t('tryIt.urlPlaceholder')" data-testid="tryit.url" />
      <n-button type="primary" size="small" :loading="sending" data-testid="tryit.send" @click="send">
        {{ sending ? t('tryIt.sending') : t('tryIt.send') }}
      </n-button>
    </div>

    <n-tabs v-model:value="activeTab" type="line" size="small" animated>
      <n-tab-pane name="headers" :tab="t('tryIt.tabHeaders')" data-testid="tryit.tab.headers">
        <api-param-table v-model="headerRows" kind="header" editable :project-id="projectId" :api-id="apiId" />
      </n-tab-pane>
      <n-tab-pane name="params" :tab="t('tryIt.tabParams')" data-testid="tryit.tab.params">
        <api-param-table v-model="queryRows" kind="request" editable :project-id="projectId" :api-id="apiId" />
      </n-tab-pane>
      <n-tab-pane name="body" :tab="t('tryIt.tabBody')" data-testid="tryit.tab.body">
        <div class="tryit-body-edit">
          <n-select v-model:value="reqBodyType" :options="bodyTypeOptions" size="small" class="tryit-body-type" data-testid="tryit.bodyType" />
          <p v-if="multipartHint" class="muted tryit-hint">{{ t('tryIt.multipartHint') }}</p>
          <n-input
            v-if="reqBodyType === 'json' || reqBodyType === 'text'"
            v-model:value="bodyText"
            type="textarea"
            :autosize="{ minRows: 5, maxRows: 14 }"
            :placeholder="t('tryIt.bodyPlaceholder')"
            data-testid="tryit.bodyText"
          />
          <template v-if="reqBodyType === 'form'">
            <div v-for="(r, i) in formRows" :key="'tf' + i" class="tryit-form-row">
              <n-input v-model:value="r.name" size="small" :placeholder="t('tryIt.formName')" data-testid="tryit.formName" />
              <n-input v-model:value="r.value" size="small" :placeholder="t('tryIt.formValue')" data-testid="tryit.formValue" />
              <n-button size="tiny" quaternary type="error" data-testid="tryit.formDel" @click="formRows.splice(i, 1)">
                {{ t('common.delete') }}
              </n-button>
            </div>
            <n-button size="tiny" data-testid="tryit.formAdd" @click="formRows.push({ name: '', value: '', enabled: true })">
              {{ t('tryIt.addRow') }}
            </n-button>
          </template>
        </div>
      </n-tab-pane>
      <n-tab-pane name="auth" :tab="t('tryIt.tabAuth')" data-testid="tryit.tab.auth">
        <div class="tryit-body-edit">
          <n-select v-model:value="authType" :options="authTypeOptions" size="small" class="tryit-auth-type" data-testid="tryit.authType" />
          <template v-if="authType === 'basic'">
            <n-input v-model:value="authUser" size="small" :placeholder="t('tryIt.authUsername')" data-testid="tryit.authUser" />
            <n-input v-model:value="authPass" size="small" type="password" show-password-on="click" :placeholder="t('tryIt.authPassword')" data-testid="tryit.authPass" />
          </template>
          <n-input v-if="authType === 'bearer'" v-model:value="authToken" size="small" type="password" show-password-on="click" :placeholder="t('tryIt.authToken')" data-testid="tryit.authToken" />
          <template v-if="authType === 'apikey'">
            <n-input v-model:value="authKey" size="small" :placeholder="t('tryIt.authKey')" data-testid="tryit.authKey" />
            <n-input v-model:value="authValue" size="small" type="password" show-password-on="click" :placeholder="t('tryIt.authValue')" data-testid="tryit.authValue" />
            <n-select v-model:value="authIn" :options="authInOptions" size="small" data-testid="tryit.authIn" />
          </template>
        </div>
      </n-tab-pane>
    </n-tabs>

    <div class="tryit-resp" data-testid="tryit.response">
      <n-spin :show="sending">
        <template v-if="resp">
          <div class="tryit-resp-head">
            <span class="tryit-status" :class="statusClass" data-testid="tryit.status">{{ resp.status }} {{ resp.status_text }}</span>
            <span class="muted">{{ t('tryIt.duration') }} {{ resp.duration_ms }} ms</span>
            <span class="muted">{{ t('tryIt.size') }} {{ fmtSize(resp.size) }}</span>
            <span v-if="resp.truncated" class="tryit-warn">{{ t('tryIt.truncated') }}</span>
            <n-button size="tiny" quaternary data-testid="tryit.copyBody" @click="copyBody">{{ t('tryIt.copyBody') }}</n-button>
          </div>
          <n-collapse v-if="resp.headers.length" class="tryit-headers">
            <n-collapse-item :title="t('tryIt.headers')" name="headers">
              <div v-for="(h, i) in resp.headers" :key="'h' + i" class="tryit-header-row">
                <span class="tryit-header-name">{{ h.name }}</span>
                <span class="tryit-header-value">{{ h.value }}</span>
              </div>
            </n-collapse-item>
          </n-collapse>
          <pre class="tryit-body" data-testid="tryit.body">{{ prettyBody }}</pre>
        </template>
        <p v-else class="muted tryit-empty">{{ t('tryIt.noResponse') }}</p>
      </n-spin>
    </div>
    </div>

    <template #footer>
      <div class="tryit-actions">
        <n-button size="small" @click="show = false">{{ t('common.close') }}</n-button>
        <n-button size="small" type="primary" :loading="sending" data-testid="tryit.sendFooter" @click="send">{{ t('tryIt.send') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.tryit-scroll {
  max-height: 70vh;
  overflow-y: auto;
  padding-right: 4px;
}

.tryit-head {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 12px;
}

.tryit-method {
  width: 104px;
}

.tryit-url {
  flex: 1 1 320px;
  min-width: 220px;
}

.tryit-body-edit {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tryit-body-type,
.tryit-auth-type {
  width: 180px;
}

.tryit-hint,
.tryit-empty {
  margin: 0;
  font-size: 12px;
}

.tryit-form-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.tryit-form-row .n-input {
  flex: 1;
}

.tryit-resp {
  margin-top: 16px;
  border-top: 1px solid var(--app-border);
  padding-top: 12px;
}

.tryit-resp-head {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  margin-bottom: 8px;
}

.tryit-status {
  font-weight: 600;
}

.tryit-status--ok {
  color: var(--app-accent);
}

.tryit-status--warn {
  color: #d48806;
}

.tryit-status--err {
  color: #d03050;
}

.tryit-warn {
  color: #d03050;
  font-size: 12px;
}

.tryit-headers {
  margin-bottom: 8px;
}

.tryit-header-row {
  display: flex;
  gap: 8px;
  font-size: 12px;
  line-height: 1.7;
}

.tryit-header-name {
  min-width: 160px;
  color: var(--app-muted);
  word-break: break-all;
}

.tryit-header-value {
  word-break: break-all;
}

.tryit-body {
  margin: 0;
  padding: 10px 12px;
  background: var(--app-bg);
  border: 1px solid var(--app-border);
  border-radius: 6px;
  max-height: 320px;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.tryit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.muted {
  color: var(--app-muted);
}
</style>
