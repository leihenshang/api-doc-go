<script setup lang="ts">
// 数据字典：外部数据库连接管理 + 表/列结构浏览（只读 introspect）。
import type { ColumnInfo, TableInfo } from '@/api/misc'
import { connColumns, connTables, createConn, deleteConn, listConns, updateConn } from '@/api/misc'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { Conn } from '@/types/entities'
import {
    NButton, NCard, NEmpty, NForm, NFormItem, NInput, NInputNumber, NModal, NSelect, NSpin, NTable, useDialog,
    useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'

const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const projectId = useNumParam('projectId')

const rows = ref<Conn[]>([])
const loading = ref(false)

const show = ref(false)
/** 编辑态：>0 表示正在编辑的连接 id，创建/编辑共用同一表单弹窗。 */
const editingId = ref(0)
const form = reactive({ address: '', port: 3306, username: '', password: '', database_name: '', type: 1, description: '' })

const showBrowse = ref(false)
const browseConn = ref<Conn | null>(null)
const tables = ref<TableInfo[]>([])
const columns = ref<ColumnInfo[]>([])
const activeTable = ref('')
const browsing = ref(false)

const typeOptions = [
    { label: 'MySQL', value: 1 },
    { label: 'PostgreSQL', value: 2 },
]
const typeLabel = (v: number): string => typeOptions.find((o) => o.value === v)?.label ?? String(v)

// 连接列表接口一次返回全量，分页在前端完成
const pageNum = ref(1)
const pageSize = 20
const paged = computed(() => rows.value.slice((pageNum.value - 1) * pageSize, pageNum.value * pageSize))
const canCreate = computed(() => project.can(2))
const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await listConns(projectId.value)
        rows.value = res.code === ErrCode.Success && res.data ? res.data : []
        // 删除后当前页可能已越界，回退到最后一页
        const pages = Math.max(1, Math.ceil(rows.value.length / pageSize))
        if (pageNum.value > pages) pageNum.value = pages
    } finally {
        loading.value = false
    }
}

function openCreate(): void {
    editingId.value = 0
    Object.assign(form, { address: '', port: 3306, username: '', password: '', database_name: '', type: 1, description: '' })
    show.value = true
}

function openEdit(c: Conn): void {
    editingId.value = c.id
    Object.assign(form, {
        address: c.address, port: c.port, username: c.username, password: '',
        database_name: c.database_name, type: c.type, description: c.description ?? '',
    })
    show.value = true
}

async function submit(): Promise<void> {
    if (!form.address.trim() || !form.database_name.trim()) {
        message.warning('地址与库名必填')
        return
    }
    const isEdit = editingId.value > 0
    const res = isEdit
        ? await updateConn({ ...form, id: editingId.value, project_id: projectId.value })
        : await createConn({ ...form, project_id: projectId.value })
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || '保存失败')
        return
    }
    message.success('已保存')
    show.value = false
    editingId.value = 0
    await load()
}

function remove(c: Conn): void {
    dialog.warning({
        title: '删除连接',
        content: `确认删除连接「${c.description || c.database_name}」？`,
        positiveText: '删除',
        negativeText: '取消',
        onPositiveClick: async () => {
            const res = await deleteConn(c.id)
            if (res.code !== ErrCode.Success) message.error(res.msg || '删除失败')
            await load()
        },
    })
}

async function openBrowse(c: Conn): Promise<void> {
    browseConn.value = c
    activeTable.value = ''
    columns.value = []
    tables.value = []
    showBrowse.value = true
    browsing.value = true
    try {
        const res = await connTables(c.id)
        if (res.code === ErrCode.Success && res.data) tables.value = res.data
        else message.error(res.msg || '读取表列表失败')
    } finally {
        browsing.value = false
    }
}

async function pickTable(name: string): Promise<void> {
    if (!browseConn.value) return
    activeTable.value = name
    browsing.value = true
    try {
        const res = await connColumns(browseConn.value.id, name)
        if (res.code === ErrCode.Success && res.data) columns.value = res.data
        else {
            columns.value = []
            message.error(res.msg || '读取列失败')
        }
    } finally {
        browsing.value = false
    }
}

onMounted(load)
</script>

<template>
    <div class="page">
        <div class="page-hd">
            <div>
                <h1 class="page-title">数据字典</h1>
                <p class="page-sub">外部数据库连接（口令加密存储）与只读表结构浏览</p>
            </div>
            <n-button v-if="canCreate" type="primary" @click="openCreate">新增连接</n-button>
        </div>

        <n-card :bordered="true" class="table-card" content-style="padding: 0">
            <n-spin :show="loading">
                <n-empty v-if="!rows.length && !loading" class="empty-block" description="还没有配置外部数据库连接" />
                <n-table v-else :single-line="false" size="medium">
                    <thead>
                        <tr>
                            <th style="width: 18%">名称</th>
                            <th style="width: 18%">地址</th>
                            <th style="width: 90px">端口</th>
                            <th style="width: 110px">类型</th>
                            <th>库名</th>
                            <th style="width: 150px">操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="c in paged" :key="c.id">
                            <td>{{ c.description || c.database_name }}</td>
                            <td class="row-mono">{{ c.address }}</td>
                            <td class="muted">{{ c.port }}</td>
                            <td class="muted">{{ typeLabel(c.type) }}</td>
                            <td class="row-mono">{{ c.database_name }}</td>
                            <td>
                                <div class="row-actions">
                                    <n-button text type="primary" size="small" @click="openBrowse(c)">浏览表</n-button>
                                    <n-button v-if="canUpdate" text size="small" @click="openEdit(c)">编辑</n-button>
                                    <n-button v-if="canDelete" text type="error" size="small"
                                        @click="remove(c)">删除</n-button>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </n-table>
            </n-spin>
            <table-pager :total="rows.length" :page-num="pageNum" :page-size="pageSize"
                @update:page-num="(v: number) => pageNum = v" />
        </n-card>

        <n-modal v-model:show="show" preset="card" :title="editingId > 0 ? '编辑连接' : '外部数据库连接'" style="width: 460px">
            <n-form label-placement="left" label-width="64">
                <n-form-item label="地址"><n-input v-model:value="form.address" placeholder="127.0.0.1" /></n-form-item>
                <n-form-item label="端口">
                    <n-input-number v-model:value="form.port" :min="1" :max="65535" style="width: 100%" />
                </n-form-item>
                <n-form-item label="用户名"><n-input v-model:value="form.username" /></n-form-item>
                <n-form-item label="密码">
                    <n-input
                        v-model:value="form.password"
                        type="password"
                        :placeholder="editingId > 0 ? '留空则沿用原口令' : '保存后 AES-256-GCM 加密存储'"
                    />
                </n-form-item>
                <n-form-item label="库名"><n-input v-model:value="form.database_name" /></n-form-item>
                <n-form-item label="类型"><n-select v-model:value="form.type" :options="typeOptions" /></n-form-item>
                <n-form-item label="备注"><n-input v-model:value="form.description" /></n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="show = false">取消</n-button>
                    <n-button type="primary" size="small" @click="submit">保存</n-button>
                </div>
            </template>
        </n-modal>

        <n-modal v-model:show="showBrowse" preset="card" :title="`表结构 · ${browseConn?.database_name ?? ''}`"
            style="width: 680px">
            <n-spin :show="browsing">
                <template v-if="!activeTable">
                    <n-empty v-if="!tables.length && !browsing" class="empty-block" description="无表或连接失败" />
                    <n-table v-else :single-line="false" size="small">
                        <thead>
                            <tr>
                                <th>表名</th>
                                <th>注释</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="t in tables" :key="t.name" class="clickable" @click="pickTable(t.name)">
                                <td class="row-mono">{{ t.name }}</td>
                                <td class="muted">{{ t.comment || '—' }}</td>
                            </tr>
                        </tbody>
                    </n-table>
                </template>
                <template v-else>
                    <div class="browse-hd">
                        <n-button size="small" @click="activeTable = ''; columns = []">← 返回表列表</n-button>
                        <span class="row-mono">{{ activeTable }}</span>
                        <span class="meta">{{ columns.length }} 列</span>
                    </div>
                    <n-table :single-line="false" size="small">
                        <thead>
                            <tr>
                                <th>列名</th>
                                <th>类型</th>
                                <th>注释</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr v-for="col in columns" :key="col.name">
                                <td class="row-mono">{{ col.name }}</td>
                                <td class="muted">{{ col.data_type }}</td>
                                <td class="muted">{{ col.comment || '—' }}</td>
                            </tr>
                        </tbody>
                    </n-table>
                </template>
            </n-spin>
        </n-modal>
    </div>
</template>

<style scoped>
.browse-hd {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
}

.foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}
</style>
