<script setup lang="ts">
// 字段映射：项目内「字段 → 类型/描述」字典，接口返回参数保存时自动回填。
import { createFieldMapping, deleteFieldMapping, listFieldMappings, updateFieldMapping } from '@/api/content'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { FieldMapping } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import {
    NButton, NCard, NEmpty, NForm, NFormItem, NInput, NModal, NSpin, NTable, useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'

const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const projectId = useNumParam('projectId')

const rows = ref<FieldMapping[]>([])
const loading = ref(false)
const keyword = ref('')

const show = ref(false)
const editId = ref(0)
const form = reactive({ field: '', type: 'string', description: '' })

const canCreate = computed(() => project.can(2))
const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))

const filtered = computed(() => {
    const k = keyword.value.trim().toLowerCase()
    if (!k) return rows.value
    return rows.value.filter((f) => `${f.field} ${f.type} ${f.description}`.toLowerCase().includes(k))
})

// 字段映射接口一次返回全量，分页在前端完成
const pageNum = ref(1)
const pageSize = 20
const paged = computed(() => filtered.value.slice((pageNum.value - 1) * pageSize, pageNum.value * pageSize))

watch(keyword, () => {
    pageNum.value = 1
})

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await listFieldMappings(projectId.value)
        rows.value = res.code === ErrCode.Success && res.data ? res.data : []
    } finally {
        loading.value = false
    }
}

function openCreate(): void {
    editId.value = 0
    Object.assign(form, { field: '', type: 'string', description: '' })
    show.value = true
}

function openEdit(f: FieldMapping): void {
    editId.value = f.id
    Object.assign(form, { field: f.field, type: f.type, description: f.description })
    show.value = true
}

async function submit(): Promise<void> {
    if (!form.field.trim() || !form.type.trim()) {
        message.warning('字段名与类型必填')
        return
    }
    const res = editId.value
        ? await updateFieldMapping({ id: editId.value, ...form, project_id: projectId.value })
        : await createFieldMapping({ ...form, project_id: projectId.value })
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || (editId.value ? '保存失败' : '添加失败'))
        return
    }
    message.success(editId.value ? '已保存' : '已添加')
    show.value = false
    await load()
}

function remove(f: FieldMapping): void {
    dialog.warning({
        title: '删除字段映射',
        content: `确认删除字段「${f.field}」？已生成的接口参数不会被改动。`,
        positiveText: '删除',
        negativeText: '取消',
        onPositiveClick: async () => {
            const res = await deleteFieldMapping(f.id)
            if (res.code !== ErrCode.Success) message.error(res.msg || '删除失败')
            await load()
        },
    })
}

onMounted(load)
</script>

<template>
    <div class="page">
        <div class="page-hd">
            <div>
                <h1 class="page-title">字段映射</h1>
                <p class="page-sub">保存接口时，返回参数按此字典自动补全类型与说明</p>
            </div>
            <n-button v-if="canCreate" type="primary" @click="openCreate">新增字段</n-button>
        </div>

        <n-card :bordered="true" class="table-card" content-style="padding: 0">
            <div class="filter">
                <n-input v-model:value="keyword" placeholder="搜索字段名 / 类型 / 描述" clearable class="search" />
            </div>
            <n-spin :show="loading">
                <n-empty v-if="!filtered.length && !loading" class="empty-block" description="暂无字段映射" />
                <n-table v-else :single-line="false" size="medium">
                    <thead>
                        <tr>
                            <th style="width: 20%">字段名</th>
                            <th style="width: 14%">类型</th>
                            <th>描述</th>
                            <th style="width: 160px">创建时间</th>
                            <th style="width: 110px">操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="f in paged" :key="f.id">
                            <td class="row-mono">{{ f.field }}</td>
                            <td class="muted">{{ f.type }}</td>
                            <td>{{ f.description || '—' }}</td>
                            <td class="muted nowrap">{{ fmtTime(f.create_time) }}</td>
                            <td>
                                <div class="row-actions">
                                    <n-button v-if="canUpdate" text size="small" @click="openEdit(f)">编辑</n-button>
                                    <n-button v-if="canDelete" text type="error" size="small"
                                        @click="remove(f)">删除</n-button>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </n-table>
            </n-spin>
            <table-pager :total="filtered.length" :page-num="pageNum" :page-size="pageSize"
                @update:page-num="(v: number) => pageNum = v" />
        </n-card>

        <n-modal v-model:show="show" preset="card" :title="editId ? '编辑字段映射' : '新增字段映射'" style="width: 420px">
            <n-form label-placement="left" label-width="64">
                <n-form-item label="字段名"><n-input v-model:value="form.field" /></n-form-item>
                <n-form-item label="类型">
                    <n-input v-model:value="form.type" placeholder="如 string / integer / object" />
                </n-form-item>
                <n-form-item label="描述"><n-input v-model:value="form.description" /></n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="show = false">取消</n-button>
                    <n-button type="primary" size="small" @click="submit">保存</n-button>
                </div>
            </template>
        </n-modal>
    </div>
</template>

<style scoped>
.filter {
    padding: 12px 16px;
    border-bottom: 1px solid var(--app-border);
}

.search {
    width: 260px;
}

.foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}
</style>
