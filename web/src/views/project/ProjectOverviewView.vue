<script setup lang="ts">
// 项目概况：基本信息 + 接口导入/导出 + 成员管理 + 操作日志（原独立页已合并至此）。
import { listUsers } from '@/api/auth'
import {
    exportOpenApi, exportPostman, importBruno, importOpenApi, importPostman, listApis, listDocs, listGroups,
    type ImportResult,
} from '@/api/content'
import { listLogs } from '@/api/misc'
import { addMember, listMembers, quitProject, setLeader, setPermission, updateProject } from '@/api/project'
import ShareLinksPanel from '@/components/ShareLinksPanel.vue'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useUserNames } from '@/composables/userNames'
import { useProjectStore } from '@/stores/project'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import type { MemberView, OpLog, User } from '@/types/entities'
import { downloadText, projectToHtml, projectToMarkdown, safeName } from '@/utils/export-doc'
import { fmtTime } from '@/utils/format'
import { EllipsisVerticalOutline } from '@vicons/ionicons5'
import type { DropdownOption, UploadFileInfo } from 'naive-ui'
import {
    NAvatar, NButton, NCard, NDropdown, NEmpty, NForm, NFormItem, NIcon, NInput, NModal, NRadio, NRadioGroup,
    NSelect, NSpin, NTable, NUpload, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { parse as parseYaml } from 'yaml'

const router = useRouter()
const message = useMessage()
const userStore = useUserStore()
const project = useProjectStore()

const projectId = useNumParam('projectId')
const { ensure: ensureUsers, nameOf } = useUserNames()

const members = ref<MemberView[]>([])
const users = ref<User[]>([])
const apiTotal = ref(0)
const loading = ref(false)

/** 负责人或管理员才可管理成员（后端 MyPermission 对负责人返回全权限位）。 */
const canManage = computed(() => userStore.isAdmin || project.permission === 15)

const pickUser = ref<number | null>(null)
const pickPerm = ref(1)
const permOptions = [
    { label: '只读', value: 1 },
    { label: '读写', value: 15 },
]
const userOptions = computed(() =>
    users.value.map((u) => ({
        label: `${u.nick_name || u.name}${u.email ? ` (${u.email})` : ''}`,
        value: u.id,
    })),
)

const showEdit = ref(false)
const form = reactive({ title: '', description: '', version: '', type: '' })

const memberMenu = (m: MemberView): DropdownOption[] => [
    { label: '设为负责人', key: 'leader', disabled: m.is_leader === 1 },
    { label: '权限：只读', key: 'p1', disabled: m.is_leader === 1 },
    { label: '权限：读写', key: 'p15', disabled: m.is_leader === 1 },
]

// ---- 操作日志（原独立页合并至此） ----
const logs = ref<OpLog[]>([])
const logTotal = ref(0)
const logPage = ref(1)
const logSize = ref(10)
const logLoading = ref(false)
const logType = ref<number | null>(null)

const typeOptions = [
    { label: '全部类型', value: 0 },
    { label: '项目', value: 1 },
    { label: '分组', value: 2 },
    { label: '接口', value: 3 },
    { label: '用户', value: 4 },
    { label: '文档', value: 5 },
]
const actionLabel: Record<string, string> = {
    create: '新增',
    update: '更新',
    delete: '删除',
    restore: '恢复',
    import: '导入',
    export: '导出',
}

async function loadLogs(): Promise<void> {
    logLoading.value = true
    try {
        const res = await listLogs({
            project_id: projectId.value,
            type: logType.value ?? 0,
            page_num: logPage.value,
            page_size: logSize.value,
        })
        if (res.code === ErrCode.Success && res.data) {
            logs.value = res.data.items
            logTotal.value = res.data.total
        }
    } finally {
        logLoading.value = false
    }
}

function reloadLogs(num = 1): void {
    logPage.value = num
    void loadLogs()
}

function onLogType(v: number | null): void {
    logType.value = v && v > 0 ? v : null
    reloadLogs(1)
}

// ---- 接口导入 / 导出 ----
const showImport = ref(false)
const importing = ref(false)
const importSrc = ref<'openapi' | 'bruno' | 'postman'>('openapi')
const importStrategy = ref<'append' | 'overwrite'>('append')
const openapiFile = ref<File | null>(null)
const postmanFile = ref<File | null>(null)
const brunoFiles = ref<{ path: string; content: string }[]>([])
const brunoReading = ref(false)
/** 目录选择会分批回调，合并成一次读取，只弹一次「共 N 个」提示。 */
let readTimer: ReturnType<typeof setTimeout> | undefined

const showResult = ref(false)
const importResult = ref<ImportResult>({ imported: 0, skipped: 0, failed: 0 })
const resultSrc = ref<'openapi' | 'bruno' | 'postman'>('openapi')
const resultStrategy = ref<'append' | 'overwrite'>('append')

/** 导入来源的中文名（结果弹窗展示）。 */
const SOURCE_LABEL: Record<'openapi' | 'bruno' | 'postman', string> = {
    openapi: 'OpenAPI 文件',
    bruno: 'Bruno 集合目录',
    postman: 'Postman 集合文件',
}

function onOpenApiFile(list: UploadFileInfo[]): void {
    openapiFile.value = list[0]?.file ?? null
}

function onPostmanFile(list: UploadFileInfo[]): void {
    postmanFile.value = list[0]?.file ?? null
}

function onBrunoDir(list: UploadFileInfo[]): void {
    if (readTimer) clearTimeout(readTimer)
    readTimer = setTimeout(() => void readBrunoDir(list), 300)
}

/** Bruno 集合目录：读入目录内全部 yml 文本（忽略 .git / node_modules）。 */
async function readBrunoDir(list: UploadFileInfo[]): Promise<void> {
    brunoReading.value = true
    try {
        const files: { path: string; content: string }[] = []
        for (const item of list) {
            const f = item.file
            if (!f) continue
            const p = f.webkitRelativePath || f.name
            if (/(^|\/)(\.git|node_modules)\//.test(p)) continue
            if (!/\.(ya?ml)$/i.test(p)) continue
            files.push({ path: p, content: await f.text() })
        }
        brunoFiles.value = files
        message.info(`共读取 ${files.length} 个文件`)
    } finally {
        brunoReading.value = false
    }
}

/** 文本解析为 OpenAPI 对象：先按 JSON，再按 YAML（含 Bruno 反引号容错）。 */
function parseOpenApi(text: string): unknown {
    try {
        return JSON.parse(text)
    } catch {
        try {
            return parseYaml(text)
        } catch {
            return parseYaml(text.replace(/^(\s*(?:- )?[\w-]+:\s*)`([^`\n]*)`(\s*)$/gm, "$1'$2'$3"))
        }
    }
}

async function doImport(): Promise<void> {
    importing.value = true
    try {
        let res
        if (importSrc.value === 'openapi') {
            if (!openapiFile.value) {
                message.warning('请选择 OpenAPI 文件')
                return
            }
            let doc: unknown
            try {
                doc = parseOpenApi(await openapiFile.value.text())
            } catch {
                message.error('OpenAPI 文件解析失败（JSON / YAML 均无法识别）')
                return
            }
            res = await importOpenApi(projectId.value, doc, 0, importStrategy.value)
        } else if (importSrc.value === 'postman') {
            if (!postmanFile.value) {
                message.warning('请选择 Postman 集合文件')
                return
            }
            let doc: unknown
            try {
                doc = JSON.parse(await postmanFile.value.text())
            } catch {
                message.error('Postman 集合解析失败（需为 Collection v2.x 导出的 JSON）')
                return
            }
            res = await importPostman(projectId.value, doc, importStrategy.value)
        } else {
            if (!brunoFiles.value.length) {
                message.warning('请选择 Bruno 集合目录')
                return
            }
            res = await importBruno(projectId.value, brunoFiles.value, importStrategy.value)
        }
        if (res.code !== ErrCode.Success) {
            message.error(res.msg || '导入失败')
            return
        }
        importResult.value = res.data ?? { imported: 0, skipped: 0, failed: 0 }
        resultSrc.value = importSrc.value
        resultStrategy.value = importStrategy.value
        showImport.value = false
        showResult.value = true
        openapiFile.value = null
        postmanFile.value = null
        brunoFiles.value = []
        await load()
    } finally {
        importing.value = false
    }
}

async function doExport(): Promise<void> {
    const res = await exportOpenApi(projectId.value, 0)
    if (res.code !== ErrCode.Success || !res.data) {
        message.error(res.msg || '导出失败')
        return
    }
    downloadJSON(res.data, `openapi-project-${projectId.value}.json`)
}

/** 导出 Postman Collection v2.1（分组 → 文件夹，接口 → 请求）。 */
async function doExportPostman(): Promise<void> {
    const res = await exportPostman(projectId.value, 0)
    if (res.code !== ErrCode.Success || !res.data) {
        message.error(res.msg || '导出失败')
        return
    }
    downloadJSON(res.data, `postman-project-${projectId.value}.json`)
}

// ---- 导出（OpenAPI / Postman / Markdown / HTML 统一入口）----
const exportOptions: DropdownOption[] = [
    { label: 'OpenAPI 3.0（.json）', key: 'openapi' },
    { label: 'Postman 集合（.json）', key: 'postman' },
    { type: 'divider', key: 'export-divider' },
    { label: 'Markdown 文档（.md）', key: 'md' },
    { label: 'HTML 文档（单文件）', key: 'html' },
]

function doExportByKey(key: string): void {
    if (key === 'openapi') {
        void doExport()
        return
    }
    if (key === 'postman') {
        void doExportPostman()
        return
    }
    void doExportDoc(key)
}

/** 导出整项目文档（Markdown / 自包含 HTML）：含全部接口与项目文档，用于对外交付与归档。 */
async function doExportDoc(kind: string): Promise<void> {
    const [apiRes, apiGroups, docGroups, docRes] = await Promise.all([
        listApis({ project_id: projectId.value, page_num: 1, page_size: 200 }),
        listGroups(projectId.value, 1),
        listGroups(projectId.value, 3),
        listDocs({ project_id: projectId.value, page_num: 1, page_size: 200 }),
    ])
    const apis = apiRes.code === ErrCode.Success && apiRes.data ? apiRes.data.items : []
    const docs = docRes.code === ErrCode.Success && docRes.data ? docRes.data.items : []
    const groups = [...(apiGroups.data ?? []), ...(docGroups.data ?? [])]
    const info = project.current ?? { title: '项目', description: '', version: '', type: '' }
    const base = safeName(project.title, `project-${projectId.value}`)
    if (kind === 'md') {
        downloadText(`${base}.md`, projectToMarkdown(info, groups, apis, docs), 'text/markdown')
        message.success(`已导出 Markdown（${apis.length} 个接口 / ${docs.length} 篇文档）`)
        return
    }
    downloadText(`${base}.html`, projectToHtml(info, groups, apis, docs), 'text/html')
    message.success(`已导出 HTML（${apis.length} 个接口 / ${docs.length} 篇文档）`)
}

/** 触发浏览器下载（JSON）。 */
function downloadJSON(data: unknown, filename: string): void {
    const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' }))
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
}

async function load(): Promise<void> {
    loading.value = true
    try {
        await Promise.all([project.load(projectId.value), ensureUsers()])
        const [m, u, a] = await Promise.all([
            listMembers(projectId.value),
            listUsers({ page_size: 200 }),
            listApis({ project_id: projectId.value, page_size: 1 }),
        ])
        if (m.code === ErrCode.Success && m.data) members.value = m.data
        if (u.code === ErrCode.Success && u.data) users.value = u.data.items
        if (a.code === ErrCode.Success && a.data) apiTotal.value = a.data.total
    } finally {
        loading.value = false
    }
}

function openEdit(): void {
    Object.assign(form, {
        title: project.current?.title ?? '',
        description: project.current?.description ?? '',
        version: project.current?.version ?? '',
        type: project.current?.type ?? '',
    })
    showEdit.value = true
}

async function submitEdit(): Promise<void> {
    if (!form.title.trim()) {
        message.warning('请输入项目名称')
        return
    }
    const res = await updateProject({ id: projectId.value, ...form })
    if (res.code !== ErrCode.Success || !res.data) {
        message.error(res.msg || '保存失败')
        return
    }
    project.set(res.data)
    message.success('已保存')
    showEdit.value = false
}

async function add(): Promise<void> {
    if (!pickUser.value) {
        message.warning('请选择用户')
        return
    }
    const res = await addMember({ project_id: projectId.value, user_id: pickUser.value, permission: pickPerm.value })
    message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '已添加成员' : res.msg)
    if (res.code === ErrCode.Success) pickUser.value = null
    await load()
}

async function onMemberAction(key: string, m: MemberView): Promise<void> {
    const res =
        key === 'leader'
            ? await setLeader({ project_id: projectId.value, user_id: m.user_id })
            : await setPermission({ project_id: projectId.value, user_id: m.user_id, permission: key === 'p1' ? 1 : 15 })
    message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '已更新' : res.msg)
    await load()
}

async function quit(): Promise<void> {
    const res = await quitProject(projectId.value)
    message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '已退出项目' : res.msg)
    if (res.code === ErrCode.Success) void router.push({ name: 'projects' })
}

function goApis(): void {
    void router.push({ name: 'projectApis', params: { projectId: projectId.value } })
}

onMounted(() => {
    void load()
    void loadLogs()
})
</script>

<template>
    <n-spin :show="loading">
        <div class="page">
            <n-card :bordered="true">
                <div class="hd">
                    <div>
                        <h1 class="title">{{ project.title || '—' }}</h1>
                        <p class="desc">{{ project.current?.description || '暂无项目描述' }}</p>
                    </div>
                    <div class="hd-actions">
                        <n-button size="small" @click="showImport = true">导入接口</n-button>
                        <n-dropdown trigger="click" :options="exportOptions" @select="doExportByKey">
                            <n-button size="small">导出文档</n-button>
                        </n-dropdown>
                        <n-button size="small" @click="openEdit">编辑</n-button>
                        <n-button v-if="!userStore.isAdmin" size="small" tertiary @click="quit">退出项目</n-button>
                    </div>
                </div>

                <ul class="info">
                    <li>
                        <span class="k">项目类型</span>
                        <span class="v">{{ project.current?.type || '—' }}</span>
                    </li>
                    <li>
                        <span class="k">项目版本</span>
                        <span class="v">{{ project.current?.version || '—' }}</span>
                    </li>
                    <li>
                        <span class="k">创建时间</span>
                        <span class="v">{{ fmtTime(project.current?.create_time) }}</span>
                    </li>
                    <li>
                        <span class="k">API 接口</span>
                        <span class="v link" @click="goApis">{{ apiTotal }} 个 · 查看详情</span>
                    </li>
                </ul>
            </n-card>

            <n-card :bordered="true" title="项目成员">
                <template #header-extra>
                    <span class="meta">共计 {{ members.length }} 位成员</span>
                </template>

                <div v-if="canManage" class="add-bar">
                    <n-select v-model:value="pickUser" :options="userOptions" placeholder="输入昵称或邮箱搜索用户" filterable
                        clearable class="add-select" />
                    <n-select v-model:value="pickPerm" :options="permOptions" class="add-perm" />
                    <n-button size="small" type="primary" @click="add">添加成员</n-button>
                </div>

                <n-empty v-if="!members.length" class="empty-block" description="还没有成员" />
                <ul v-else class="members">
                    <li v-for="m in members" :key="m.user_id" class="member">
                        <n-avatar round :size="36" class="avatar">
                            {{ (m.nick_name || m.name || 'U').slice(0, 1).toUpperCase() }}
                        </n-avatar>
                        <div class="m-info">
                            <div class="m-name ellipsis" :title="m.nick_name || m.name">
                                {{ m.nick_name || m.name }}
                                <span v-if="m.user_id === (userStore.userInfo?.id ?? 0)" class="meta">（我）</span>
                            </div>
                            <div class="m-role">
                                {{ m.is_leader === 1 ? '负责人' : m.permission === 15 ? '成员（读写）' : '成员（只读）' }}
                            </div>
                            <div class="m-mail ellipsis">{{ m.name }}</div>
                        </div>
                        <n-dropdown v-if="canManage" trigger="click" :options="memberMenu(m)"
                            @select="onMemberAction($event, m)">
                            <span class="more"><n-icon :component="EllipsisVerticalOutline" /></span>
                        </n-dropdown>
                    </li>
                </ul>
            </n-card>

            <!-- 操作日志：由独立页合并而来 -->
            <!-- 分享链接：免登录只读入口 -->
            <share-links-panel :project-id="projectId" />

            <n-card :bordered="true" title="操作日志" content-style="padding: 0">
                <template #header-extra>
                    <div class="log-hd">
                        <span class="meta">共 {{ logTotal }} 条 · 保留 70 天</span>
                        <n-select :value="logType" :options="typeOptions" placeholder="对象类型" class="log-type"
                            @update:value="onLogType" />
                    </div>
                </template>
                <n-spin :show="logLoading">
                    <n-empty v-if="!logs.length && !logLoading" class="empty-block" description="暂无日志" />
                    <n-table v-else :single-line="false" size="small" class="log-table">
                        <thead>
                            <tr>
                                <th style="width: 170px">时间</th>
                                <th style="width: 130px">操作人</th>
                                <th style="width: 80px">动作</th>
                                <th style="width: 100px">对象类型</th>
                                <th>内容</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="l in logs" :key="l.id">
                                <td class="muted">{{ fmtTime(l.create_time) }}</td>
                                <td>{{ nameOf(l.user_id) }}</td>
                                <td>{{ actionLabel[l.action] ?? l.action }}</td>
                                <td class="muted">{{ typeOptions.find((o) => o.value === l.type)?.label ?? l.type }}</td>
                                <td class="ellipsis" :title="l.content">{{ l.content || '—' }}</td>
                            </tr>
                        </tbody>
                    </n-table>
                </n-spin>
                <table-pager :total="logTotal" :page-num="logPage" :page-size="logSize" @update:page-num="reloadLogs" />
            </n-card>
        </div>

        <n-modal v-model:show="showEdit" preset="card" title="编辑项目" style="width: 460px">
            <n-form label-placement="left" label-width="72">
                <n-form-item label="名称"><n-input v-model:value="form.title" /></n-form-item>
                <n-form-item label="版本号"><n-input v-model:value="form.version" /></n-form-item>
                <n-form-item label="类型"><n-input v-model:value="form.type" /></n-form-item>
                <n-form-item label="描述">
                    <n-input v-model:value="form.description" type="textarea" :rows="3" />
                </n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="showEdit = false">取消</n-button>
                    <n-button type="primary" size="small" @click="submitEdit">保存</n-button>
                </div>
            </template>
        </n-modal>

        <n-modal v-model:show="showImport" preset="card" title="导入接口" style="width: 560px">
            <n-form label-placement="left" label-width="88">
                <n-form-item label="数据来源">
                    <n-radio-group v-model:value="importSrc">
                        <n-radio value="openapi">OpenAPI 文件（JSON / YAML）</n-radio>
                        <n-radio value="bruno">Bruno 集合目录</n-radio>
                        <n-radio value="postman">Postman 集合文件（JSON）</n-radio>
                    </n-radio-group>
                </n-form-item>
                <n-form-item label="导入策略">
                    <n-radio-group v-model:value="importStrategy">
                        <n-radio value="append">只新增（不覆盖）</n-radio>
                        <n-radio value="overwrite">完全覆盖当前项目</n-radio>
                    </n-radio-group>
                </n-form-item>
                <n-form-item label="选择内容">
                    <n-upload v-if="importSrc === 'bruno'" directory :default-upload="false" :show-file-list="false"
                        @update:file-list="onBrunoDir">
                        <n-button>选择 Bruno 集合目录</n-button>
                    </n-upload>
                    <n-upload v-else-if="importSrc === 'postman'" :max="1" :default-upload="false" accept=".json"
                        @update:file-list="onPostmanFile">
                        <n-button>选择 Postman 集合文件</n-button>
                    </n-upload>
                    <n-upload v-else :max="1" :default-upload="false" accept=".json,.yaml,.yml"
                        @update:file-list="onOpenApiFile">
                        <n-button>选择 OpenAPI 文件</n-button>
                    </n-upload>
                </n-form-item>
            </n-form>

            <!-- Bruno 文件清单：定高 + 滚动，避免目录文件多时撑爆弹窗 -->
            <div v-if="importSrc === 'bruno'" class="flist">
                <div class="flist-hd">
                    <span>已读取 <b>{{ brunoFiles.length }}</b> 个文件</span>
                    <span v-if="brunoReading" class="meta">读取中…</span>
                </div>
                <ul class="flist-body">
                    <li v-for="f in brunoFiles" :key="f.path" class="ellipsis" :title="f.path">{{ f.path }}</li>
                    <li v-if="!brunoFiles.length" class="meta">尚未选择目录</li>
                </ul>
            </div>

            <p v-if="importSrc === 'bruno'" class="meta">
                选择 Bruno 集合根目录（内含 opencollection.yml），目录映射为接口分组，请求映射为接口。
            </p>
            <p v-else-if="importSrc === 'postman'" class="meta">
                选择 Postman 导出的 Collection（v2.x JSON），目录映射为接口分组，请求映射为接口；查询参数与 JSON 请求体会展平为请求参数。
            </p>
            <p class="meta warn">
                「完全覆盖」会先清空当前项目的全部接口与接口目录，再导入所选内容，不可恢复，请谨慎操作。
            </p>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="showImport = false">取消</n-button>
                    <n-button type="primary" size="small" :loading="importing" @click="doImport">导入</n-button>
                </div>
            </template>
        </n-modal>

        <!-- 导入结果：成功 / 跳过 / 失败 + 失败明细，确认后关闭 -->
        <n-modal v-model:show="showResult" preset="card" title="导入结果" style="width: 520px">
            <p class="res-src meta">
                来源：{{ SOURCE_LABEL[resultSrc] }} · 策略：{{ resultStrategy === 'overwrite' ? '完全覆盖' : '只新增' }}
            </p>
            <ul class="res-list">
                <li>
                    <span class="dot dot--ok" />
                    <span>成功导入</span>
                    <b class="ok">{{ importResult.imported }}</b>
                    <span>个接口</span>
                </li>
                <li v-if="importResult.skipped">
                    <span class="dot dot--skip" />
                    <span>已存在跳过</span>
                    <b>{{ importResult.skipped }}</b>
                    <span>个接口</span>
                </li>
                <li v-if="importResult.failed">
                    <span class="dot dot--err" />
                    <span>导入失败</span>
                    <b class="err">{{ importResult.failed }}</b>
                    <span>个文件</span>
                </li>
            </ul>
            <p v-if="!importResult.imported && !importResult.skipped && !importResult.failed" class="meta">
                未发现可导入的接口，请确认所选内容。
            </p>

            <div v-if="importResult.failures?.length" class="res-failures">
                <p class="meta">失败明细（最多显示 10 条）：</p>
                <ul class="res-failures-body">
                    <li v-for="f in importResult.failures" :key="f" class="ellipsis" :title="f">{{ f }}</li>
                </ul>
            </div>

            <p v-if="resultStrategy === 'overwrite'" class="meta warn">已按「完全覆盖」策略清空原有接口与接口目录。</p>
            <template #footer>
                <div class="foot">
                    <n-button type="primary" size="small" @click="showResult = false">确认</n-button>
                </div>
            </template>
        </n-modal>
    </n-spin>
</template>

<style scoped>
.hd {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 16px;
}

.title {
    margin: 0;
    font-size: 22px;
    font-weight: 600;
    color: var(--app-accent);
}

.desc {
    margin: 8px 0 16px;
    color: var(--app-muted);
}

.hd-actions {
    display: flex;
    gap: 8px;
}

.info {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    border: 1px solid var(--app-border);
}

.info li {
    padding: 12px 16px;
    border-right: 1px solid var(--app-border);
}

.info li:last-child {
    border-right: 0;
}

.k {
    display: block;
    font-size: 12px;
    color: var(--app-muted);
    margin-bottom: 6px;
}

.v {
    font-size: 14px;
}

.add-bar {
    display: flex;
    gap: 8px;
    margin-bottom: 16px;
}

.add-select {
    width: 280px;
}

.add-perm {
    width: 110px;
}

.members {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 12px;
}

.member {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px;
    border: 1px solid var(--app-border);
    border-radius: 4px;
}

.avatar {
    background: var(--app-accent);
    color: #fff;
    flex: 0 0 auto;
}

.m-info {
    flex: 1;
    min-width: 0;
}

.m-name {
    font-weight: 500;
}

.m-role {
    font-size: 12px;
    color: var(--app-muted);
}

.m-mail {
    font-size: 12px;
    color: var(--app-muted);
}

.more {
    display: inline-flex;
    padding: 4px;
    border-radius: 3px;
    color: var(--app-muted);
    cursor: pointer;
}

.more:hover {
    background: #eceff1;
    color: var(--app-text);
}

.log-hd {
    display: flex;
    align-items: center;
    gap: 12px;
}

.log-type {
    width: 130px;
}

.log-table {
    min-width: 720px;
}

.foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}

.warn {
    margin: 8px 0 0;
    color: #d46b08;
}

/* Bruno 文件清单：定高 + 内滚动 */
.flist {
    margin-bottom: 12px;
    border: 1px solid var(--app-border);
    border-radius: 4px;
    overflow: hidden;
}

.flist-hd {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 10px;
    background: #f7f8fa;
    font-size: 12px;
    border-bottom: 1px solid var(--app-border);
}

.flist-body {
    list-style: none;
    margin: 0;
    padding: 6px 10px;
    height: 132px;
    overflow: auto;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
}

.flist-body li {
    padding: 1px 0;
}

/* 导入结果 */
.res-src {
    margin: 0 0 12px;
}

.res-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 13px;
}

.res-list li {
    display: flex;
    align-items: center;
    gap: 8px;
}

.res-list b {
    font-size: 15px;
}

.dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex: 0 0 auto;
}

.dot--ok {
    background: var(--app-accent);
}

.dot--skip {
    background: #b9bfc6;
}

.dot--err {
    background: #eb2013;
}

.ok {
    color: var(--app-accent);
}

.err {
    color: #eb2013;
}

.res-failures {
    margin-top: 14px;
    padding-top: 12px;
    border-top: 1px solid var(--app-border);
}

.res-failures-body {
    list-style: none;
    margin: 6px 0 0;
    padding: 0;
    max-height: 120px;
    overflow: auto;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12px;
    color: var(--app-muted);
}

.res-failures-body li {
    padding: 1px 0;
}

@media (max-width: 900px) {
    .info {
        grid-template-columns: repeat(2, 1fr);
    }

    .hd,
    .hd-actions {
        flex-direction: column;
        align-items: flex-start;
    }
}
</style>
