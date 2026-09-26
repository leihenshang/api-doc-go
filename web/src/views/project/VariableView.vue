<script setup lang="ts">
// 变量：项目级环境管理 + 三级变量（请求级/项目级/全局）维护。
// 项目变量按环境取值（通用 env_id=0 为兜底）；请求变量仅对所属接口生效；
// 全局变量为应用级（跨项目共享），在「公共属性」页维护，此处只读展示。
import { listApis } from '@/api/content'
import {
    createVariable, createVariableEnv, deleteVariable, deleteVariableEnv, listVariableEnvs,
    listVariables, setDefaultVariableEnv, updateVariable, updateVariableEnv, variableEnvRefs,
} from '@/api/variable'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { useVariableStore } from '@/stores/variable'
import { useI18n } from 'vue-i18n'
import { ErrCode } from '@/types/api'
import type { ApiDoc, Variable, VariableEnv } from '@/types/entities'
import {
    NButton, NCard, NCheckbox, NEmpty, NForm, NFormItem, NInput, NInputNumber, NModal, NSelect,
    NSpin, NTable, NTag, useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'

/** 变量作用域：1 请求级（API）/ 2 项目级 / 3 全局（应用级）。 */
const Scope = { Request: 1, Project: 2, Global: 3 } as const

/** 变量名规则与后端一致：字母/下划线开头，仅含字母数字下划线，最长 64 位。 */
const NAME_RE = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/

/** 占位符示例（避免在模板里直接写花括号导致插值解析歧义）。 */
const placeholderHint = '{{name}}'

const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const variableStore = useVariableStore()
const { t } = useI18n()
const projectId = useNumParam('projectId')

const envs = ref<VariableEnv[]>([])
const envCounts = ref<Record<number, number>>({})
const rows = ref<Variable[]>([])
const apis = ref<ApiDoc[]>([])
const loading = ref(false)

// 筛选：作用域 / 环境（'all' 全部、'common' 通用、数字=某环境）/ 接口 / 关键词
const filterScope = ref<number>(Scope.Project)
const filterEnv = ref<string | number>('all')
const filterApiId = ref(0)
const keyword = ref('')

const pageNum = ref(1)
const pageSize = 20
const paged = computed(() => rows.value.slice((pageNum.value - 1) * pageSize, pageNum.value * pageSize))

const canCreate = computed(() => project.can(2))
const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))
/** 全局变量在「公共属性」页维护（后续阶段），此处只读。 */
const canCreateHere = computed(() => canCreate.value && filterScope.value !== Scope.Global)

const scopeOptions = [
    { label: t('variable.scopeProject'), value: Scope.Project },
    { label: t('variable.scopeRequest'), value: Scope.Request },
    { label: t('variable.scopeGlobal'), value: Scope.Global },
]
const envFilterOptions = computed(() => [
    { label: t('variable.scopeAll'), value: 'all' },
    { label: t('variable.scopeCommon'), value: 'common' },
    ...envs.value.map((e) => ({ label: e.is_default === 1 ? t('variable.scopeDefaultEnv', { name: e.name }) : e.name, value: e.id as number })),
])
const varEnvOptions = computed(() => [
    { label: t('variable.scopeCommon'), value: 0 },
    ...envs.value.map((e) => ({ label: e.name, value: e.id as number })),
])
const apiOptions = computed(() => [
    { label: t('variable.apiAll'), value: 0 },
    ...apis.value.map((a) => ({ label: `${a.http_method_type} ${a.api_name}`, value: a.id })),
])

// ---- 环境弹窗 ----
const envShow = ref(false)
const envEditId = ref(0)
const envForm = reactive({ name: '', description: '', color: '#18a058', priority: 0 })

// ---- 变量弹窗 ----
const varShow = ref(false)
const varEditId = ref(0)
/** 编辑中变量的载入时间（乐观锁期望值）。 */
const varStamp = ref('')
const varForm = reactive({
    scope: Scope.Project as number,
    name: '',
    value: '',
    description: '',
    env_id: 0,
    api_id: 0,
    is_secret: false,
    enabled: true,
})

function scopeText(scope: number): string {
    if (scope === Scope.Request) return t('variable.scopeRequest')
    if (scope === Scope.Project) return t('variable.scopeProject')
    return t('variable.scopeGlobalPlain')
}

function envName(envId: number): string {
    if (envId === 0) return t('variable.commonEnv')
    return envs.value.find((e) => e.id === envId)?.name ?? '—'
}

function apiName(apiId: number): string {
    const hit = apis.value.find((a) => a.id === apiId)
    return hit ? hit.api_name : `#${apiId}`
}

/** 归属列：请求变量显示接口，项目变量显示环境，全局变量显示全部项目。 */
function ownerText(v: Variable): string {
    if (v.scope === Scope.Request) return apiName(v.api_id)
    if (v.scope === Scope.Project) return envName(v.env_id)
    return t('variable.projectAll')
}

function canEditVar(v: Variable): boolean {
    return v.scope !== Scope.Global && canUpdate.value
}

function canDeleteVar(v: Variable): boolean {
    if (v.scope === Scope.Global) return false
    return v.scope === Scope.Request ? canUpdate.value : canDelete.value
}

async function loadEnvs(): Promise<void> {
    const res = await listVariableEnvs(projectId.value)
    envs.value = res.code === ErrCode.Success && res.data ? res.data : []
}

/** 各环境的变量数量（用于环境胶囊上的计数，独立于当前筛选条件）。 */
async function loadEnvCounts(): Promise<void> {
    const res = await listVariables({ project_id: projectId.value, scope: Scope.Project })
    const list = res.code === ErrCode.Success && res.data ? res.data : []
    const map: Record<number, number> = {}
    for (const v of list) map[v.env_id] = (map[v.env_id] ?? 0) + 1
    envCounts.value = map
}

async function loadApis(): Promise<void> {
    if (apis.value.length) return
    const res = await listApis({ project_id: projectId.value, page_num: 1, page_size: 200 })
    apis.value = res.code === ErrCode.Success && res.data ? res.data.items : []
}

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await listVariables({
            project_id: projectId.value,
            scope: filterScope.value,
            api_id: filterApiId.value || undefined,
            env_id: filterEnv.value === 'all' ? undefined : filterEnv.value === 'common' ? 0 : Number(filterEnv.value),
            keyword: keyword.value.trim() || undefined,
        })
        rows.value = res.code === ErrCode.Success && res.data ? res.data : []
    } finally {
        loading.value = false
    }
}

/** 变量/环境变更后：清掉详情页与左树的解析缓存，并刷新共享环境列表（当前环境被删时自动回落默认）。 */
async function reloadAll(): Promise<void> {
    variableStore.invalidate(projectId.value)
    await Promise.all([loadEnvs(), loadEnvCounts(), load(), variableStore.loadEnvs(projectId.value, true)])
}

let keywordTimer: ReturnType<typeof setTimeout> | undefined
watch(keyword, () => {
    clearTimeout(keywordTimer)
    keywordTimer = setTimeout(() => {
        pageNum.value = 1
        void load()
    }, 300)
})

watch([filterScope, filterEnv, filterApiId], () => {
    pageNum.value = 1
    if (filterScope.value === Scope.Request) void loadApis()
    void load()
})

onMounted(async () => {
    await reloadAll()
    if (filterScope.value === Scope.Request) await loadApis()
})

// ---- 环境操作 ----

function openEnvCreate(): void {
    envEditId.value = 0
    Object.assign(envForm, { name: '', description: '', color: '#18a058', priority: 0 })
    envShow.value = true
}

function openEnvEdit(env: VariableEnv): void {
    envEditId.value = env.id
    Object.assign(envForm, {
        name: env.name,
        description: env.description,
        color: env.color || '#18a058',
        priority: env.priority,
    })
    envShow.value = true
}

async function submitEnv(): Promise<void> {
    const name = envForm.name.trim()
    if (!name) {
        message.warning(t('variable.envRequired'))
        return
    }
    const payload = { name, description: envForm.description, color: envForm.color, priority: envForm.priority }
    const res = envEditId.value
        ? await updateVariableEnv({ id: envEditId.value, project_id: projectId.value, ...payload })
        : await createVariableEnv({ project_id: projectId.value, ...payload })
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.saveFailed'))
        return
    }
    message.success(envEditId.value ? t('common.saved') : t('variable.envCreated'))
    envShow.value = false
    await reloadAll()
}

async function markDefault(env: VariableEnv): Promise<void> {
    const res = await setDefaultVariableEnv(env.id)
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('variable.envSetFailed'))
        return
    }
    message.success(t('variable.envDefaultSet', { name: env.name }))
    await reloadAll()
}

/** 删除环境前先预检引用：默认环境或仍被变量引用时只提示、不进入删除确认。 */
async function removeEnv(env: VariableEnv): Promise<void> {
    const res = await variableEnvRefs(env.id)
    if (res.code !== ErrCode.Success || !res.data) {
        message.error(res.msg || t('variable.envRefsFailed'))
        return
    }
    const refs = res.data
    if (refs.blocked) {
        const parts: string[] = []
        if (refs.env.is_default === 1) parts.push(t('variable.envDeleteBlockedDefault'))
        if (refs.variables.length) {
            parts.push(t('variable.envDeleteBlockedRefs', { n: refs.variables.length, names: refs.variables.map((v) => v.name).join(t('common.listSep')) }))
        }
        dialog.warning({
            title: t('variable.envDeleteBlockedTitle', { name: env.name }),
            content: parts.join(t('common.sentenceSep')) || t('variable.envDeleteBlockedFallback'),
            positiveText: t('common.gotIt'),
        })
        return
    }
    dialog.warning({
        title: t('variable.envDeleteTitle', { name: env.name }),
        content: t('variable.envDeleteConfirm'),
        positiveText: t('common.delete'),
        negativeText: t('common.cancel'),
        onPositiveClick: async () => {
            const del = await deleteVariableEnv(env.id)
            if (del.code !== ErrCode.Success) {
                message.error(del.msg || t('common.deleteFailed'))
                return false
            }
            message.success(t('common.deleted'))
            if (filterEnv.value === env.id) filterEnv.value = 'all'
            await reloadAll()
        },
    })
}

function toggleEnvFilter(env: VariableEnv): void {
    filterEnv.value = filterEnv.value === env.id ? 'all' : env.id
}

// ---- 变量操作 ----

function openCreate(): void {
    varEditId.value = 0
    varStamp.value = ''
    const scope = filterScope.value === Scope.Request ? Scope.Request : Scope.Project
    const defaultEnv = envs.value.find((e) => e.is_default === 1)?.id ?? envs.value[0]?.id ?? 0
    Object.assign(varForm, {
        scope,
        name: '',
        value: '',
        description: '',
        env_id: filterEnv.value === 'common' ? 0 : typeof filterEnv.value === 'number' ? filterEnv.value : defaultEnv,
        api_id: scope === Scope.Request ? filterApiId.value : 0,
        is_secret: false,
        enabled: true,
    })
    varShow.value = true
}

/** 敏感值列表只回掩码，编辑时需带 with_value=1 单独取明文（后端记录操作日志）。 */
async function fetchPlainValue(row: Variable): Promise<string> {
    if (row.is_secret !== 1) return row.value
    const res = await listVariables({
        project_id: projectId.value,
        scope: row.scope,
        api_id: row.api_id || undefined,
        keyword: row.name,
        with_value: 1,
    })
    const hit = res.code === ErrCode.Success && res.data ? res.data.find((v) => v.id === row.id) : undefined
    return hit ? hit.value : ''
}

async function openEdit(row: Variable): Promise<void> {
    if (row.scope === Scope.Request) await loadApis()
    const value = await fetchPlainValue(row)
    varEditId.value = row.id
    // 乐观锁：保存时回传载入时间，服务端据此判断是否被他人改过。
    varStamp.value = row.update_time
    Object.assign(varForm, {
        scope: row.scope,
        name: row.name,
        value,
        description: row.description,
        env_id: row.env_id,
        api_id: row.api_id,
        is_secret: row.is_secret === 1,
        enabled: row.enabled === 1,
    })
    varShow.value = true
}

async function submitVar(): Promise<void> {
    const name = varForm.name.trim()
    if (!NAME_RE.test(name)) {
        message.warning(t('varForm.nameInvalid'))
        return
    }
    if (varForm.scope === Scope.Request && !varForm.api_id) {
        message.warning(t('variable.requestRequired'))
        return
    }
    const payload = {
        scope: varForm.scope,
        project_id: projectId.value,
        api_id: varForm.scope === Scope.Request ? varForm.api_id : 0,
        env_id: varForm.scope === Scope.Project ? varForm.env_id : 0,
        name,
        value: varForm.value,
        description: varForm.description,
        is_secret: varForm.is_secret ? 1 : 0,
        enabled: varForm.enabled ? 1 : 0,
    }
    const res = varEditId.value
        ? await updateVariable({ id: varEditId.value, ...payload, expected_update_time: varStamp.value })
        : await createVariable(payload)
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || (varEditId.value ? t('common.saveFailed') : t('common.createFailed')))
        // 冲突：刷新列表，避免继续在旧数据上编辑。
        if ((res.msg || '').includes('已被他人修改')) await reloadAll()
        return
    }
    message.success(varEditId.value ? t('common.saved') : t('variable.created'))
    varShow.value = false
    await reloadAll()
}

function removeVar(row: Variable): void {
    const tip = row.scope === Scope.Request
        ? t('variable.deleteTipApi')
        : t('variable.deleteTipOther')
    dialog.warning({
        title: t('variable.deleteTitle', { name: row.name }),
        content: t('variable.deleteConfirm', { tip }),
        positiveText: t('common.delete'),
        negativeText: t('common.cancel'),
        onPositiveClick: async () => {
            const res = await deleteVariable(row.id)
            if (res.code !== ErrCode.Success) {
                message.error(res.msg || t('common.deleteFailed'))
                return false
            }
            message.success(t('common.deleted'))
            await reloadAll()
        },
    })
}
</script>

<template>
    <div class="page">
        <div class="page-hd">
            <div>
                <h1 class="page-title">{{ t('variable.title') }}</h1>
                <p class="page-sub">
                    {{ t('variable.subBefore') }}<code>{{ placeholderHint }}</code>
                </p>
            </div>
            <n-button v-if="canCreateHere" type="primary" @click="openCreate">{{ t('variable.create') }}</n-button>
        </div>

        <!-- 环境条：点击胶囊按环境筛选，胶囊上可编辑/设默认/删除 -->
        <n-card :bordered="true" class="env-card">
            <div class="env-bar">
                <span class="env-label">{{ t('variable.envLabel') }}</span>
                <div class="env-list">
                    <div
                        v-for="e in envs"
                        :key="e.id"
                        class="env-pill"
                        :class="{ 'env-pill--active': filterEnv === e.id }"
                        :title="t('variable.envTip', { name: e.name, n: envCounts[e.id] ?? 0 })"
                        @click="toggleEnvFilter(e)"
                    >
                        <span class="dot" :style="{ background: e.color || '#18a058' }" />
                        <span class="env-name">{{ e.name }}</span>
                        <n-tag v-if="e.is_default === 1" size="tiny" :bordered="false" type="success">{{ t('variable.defaultTag') }}</n-tag>
                        <span class="env-count">{{ envCounts[e.id] ?? 0 }}</span>
                        <span v-if="canUpdate" class="env-actions">
                            <n-button text size="tiny" @click.stop="openEnvEdit(e)">{{ t('common.edit') }}</n-button>
                            <n-button v-if="e.is_default !== 1" text size="tiny" @click.stop="markDefault(e)">{{ t('variable.setDefault') }}</n-button>
                            <n-button v-if="canDelete" text size="tiny" type="error" @click.stop="removeEnv(e)">{{ t('common.delete') }}</n-button>
                        </span>
                    </div>
                    <span v-if="!envs.length" class="muted">{{ t('variable.noEnv') }}</span>
                    <n-button v-if="canCreate" text size="small" type="primary" @click="openEnvCreate">{{ t('variable.createEnv') }}</n-button>
                </div>
            </div>
        </n-card>

        <n-card :bordered="true" class="table-card" content-style="padding: 0">
            <div class="filter">
                <n-select v-model:value="filterScope" :options="scopeOptions" class="f-scope" />
                <n-select v-model:value="filterEnv" :options="envFilterOptions" class="f-env" />
                <n-select v-if="filterScope === Scope.Request" v-model:value="filterApiId" :options="apiOptions" class="f-api" />
                <n-input v-model:value="keyword" :placeholder="t('variable.searchPlaceholder')" clearable class="search" />
            </div>
            <n-spin :show="loading">
                <n-empty v-if="!rows.length && !loading" class="empty-block" :description="t('variable.empty')" />
                <n-table v-else :single-line="false" size="medium">
                    <thead>
                        <tr>
                            <th style="width: 18%">{{ t('varForm.name') }}</th>
                            <th style="width: 22%">{{ t('varForm.value') }}</th>
                            <th>{{ t('field.description') }}</th>
                            <th style="width: 16%">{{ t('variable.owner') }}</th>
                            <th style="width: 11%">{{ t('variable.scopeLabel') }}</th>
                            <th style="width: 80px">{{ t('varForm.status') }}</th>
                            <th style="width: 120px">{{ t('api.actions') }}</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="v in paged" :key="v.id">
                            <td class="row-mono">{{ v.name }}</td>
                            <td class="row-mono">
                                <span v-if="v.is_secret === 1" class="masked" :title="t('varForm.maskedTip')">{{ v.has_value ? '••••••' : t('varForm.emptySecret') }}</span>
                                <span v-else-if="v.has_value">{{ v.value }}</span>
                                <span v-else class="muted">{{ t('varForm.emptyValue') }}</span>
                            </td>
                            <td>{{ v.description || '—' }}</td>
                            <td>{{ ownerText(v) }}</td>
                            <td>{{ scopeText(v.scope) }}</td>
                            <td>{{ v.enabled === 1 ? t('varForm.enabled') : t('varForm.disabled') }}</td>
                            <td>
                                <div class="row-actions">
                                    <n-button v-if="canEditVar(v)" text size="small" @click="openEdit(v)">{{ t('common.edit') }}</n-button>
                                    <n-button v-if="canDeleteVar(v)" text type="error" size="small" @click="removeVar(v)">{{ t('common.delete') }}</n-button>
                                    <span v-if="v.scope === Scope.Global" class="muted tiny">{{ t('variable.ownedByProperty') }}</span>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </n-table>
            </n-spin>
            <table-pager :total="rows.length" :page-num="pageNum" :page-size="pageSize"
                @update:page-num="(v: number) => pageNum = v" />
        </n-card>

        <!-- 变量弹窗 -->
        <n-modal v-model:show="varShow" preset="card" :title="varEditId ? t('variable.varEditTitle') : t('variable.varCreateTitle')" style="width: 480px">
            <n-form label-placement="left" label-width="86">
                <n-form-item :label="t('variable.scopeLabel')">
                    <span class="muted">{{ scopeText(varForm.scope) }}</span>
                </n-form-item>
                <n-form-item v-if="varForm.scope === Scope.Request" :label="t('variable.apiLabel')">
                    <n-select v-model:value="varForm.api_id" :options="apiOptions.filter((o) => o.value !== 0)" filterable
                        :placeholder="t('variable.apiPlaceholder')" />
                </n-form-item>
                <n-form-item v-if="varForm.scope === Scope.Project" :label="t('variable.envField')">
                    <n-select v-model:value="varForm.env_id" :options="varEnvOptions" />
                </n-form-item>
                <n-form-item :label="t('varForm.name')">
                    <n-input v-model:value="varForm.name" :placeholder="t('varForm.namePlaceholder')" />
                </n-form-item>
                <n-form-item :label="t('varForm.value')">
                    <n-input v-model:value="varForm.value" type="textarea" :rows="2" :placeholder="t('varForm.valuePlaceholder')" />
                </n-form-item>
                <n-form-item :label="t('field.description')">
                    <n-input v-model:value="varForm.description" :placeholder="t('common.optional')" />
                </n-form-item>
                <n-form-item :label="t('variable.options')">
                    <div class="opts">
                        <n-checkbox v-model:checked="varForm.is_secret">{{ t('varForm.secret') }}</n-checkbox>
                        <n-checkbox v-model:checked="varForm.enabled">{{ t('varForm.enable') }}</n-checkbox>
                    </div>
                </n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="varShow = false">{{ t('common.cancel') }}</n-button>
                    <n-button type="primary" size="small" @click="submitVar">{{ t('common.save') }}</n-button>
                </div>
            </template>
        </n-modal>

        <!-- 环境弹窗 -->
        <n-modal v-model:show="envShow" preset="card" :title="envEditId ? t('variable.envEditTitle') : t('variable.envCreateTitle')" style="width: 420px">
            <n-form label-placement="left" label-width="72">
                <n-form-item :label="t('variable.envName')">
                    <n-input v-model:value="envForm.name" :placeholder="t('variable.envNamePlaceholder')" />
                </n-form-item>
                <n-form-item :label="t('field.description')">
                    <n-input v-model:value="envForm.description" :placeholder="t('common.optional')" />
                </n-form-item>
                <n-form-item :label="t('variable.envColor')">
                    <n-input v-model:value="envForm.color" placeholder="#18a058" />
                </n-form-item>
                <n-form-item :label="t('variable.envSort')">
                    <n-input-number v-model:value="envForm.priority" :min="0" />
                </n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="envShow = false">{{ t('common.cancel') }}</n-button>
                    <n-button type="primary" size="small" @click="submitEnv">{{ t('common.save') }}</n-button>
                </div>
            </template>
        </n-modal>
    </div>
</template>

<style scoped>
.env-card {
    margin-bottom: var(--app-gap);
}

.env-bar {
    display: flex;
    align-items: flex-start;
    gap: 12px;
}

.env-label {
    flex: 0 0 auto;
    padding-top: 4px;
    font-size: 13px;
    color: var(--app-muted);
}

.env-list {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
}

.env-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: 30px;
    padding: 0 10px;
    border: 1px solid var(--app-border);
    border-radius: 15px;
    font-size: 13px;
    cursor: pointer;
    user-select: none;
    transition: border-color 0.15s, background 0.15s;
}

.env-pill:hover {
    border-color: var(--app-accent);
}

.env-pill--active {
    border-color: var(--app-accent);
    background: rgba(24, 160, 88, 0.08);
}

.env-pill .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
}

.env-name {
    font-weight: 600;
}

.env-count {
    min-width: 18px;
    padding: 0 5px;
    border-radius: 9px;
    background: #f0f1f3;
    color: var(--app-muted);
    font-size: 12px;
    text-align: center;
}

.env-actions {
    display: none;
    align-items: center;
    gap: 6px;
    padding-left: 6px;
    border-left: 1px solid var(--app-border);
}

.env-pill:hover .env-actions {
    display: inline-flex;
}

.filter {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--app-border);
}

.f-scope {
    width: 150px;
}

.f-env {
    width: 168px;
}

.f-api {
    width: 220px;
}

.search {
    width: 240px;
    margin-left: auto;
}

.masked {
    letter-spacing: 1px;
    color: var(--app-muted);
}

.opts {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.tiny {
    font-size: 12px;
}

.foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}

/* ---- 移动端：筛选栏换行、下拉占满宽度 ---- */
@media (max-width: 768px) {
    .filter {
        flex-wrap: wrap;
    }

    .f-scope,
    .f-env,
    .f-api,
    .search {
        width: 100%;
        margin-left: 0;
    }
}
</style>
