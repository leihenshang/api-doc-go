<script setup lang="ts">
// 备忘清单：列表（支持置顶）、新建、状态流转、指派处理人、删除。
import { listUsers } from '@/api/auth'
import { assignBug, createBug, deleteBug, listBugAssign, listBugs, updateBug } from '@/api/misc'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { Bug, BugAssign, User } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import {
    NButton, NCard, NEmpty, NForm, NFormItem, NInput, NModal, NSelect, NSpin, NTable, NTag, useDialog,
    useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'

const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const projectId = useNumParam('projectId')

const rows = ref<Bug[]>([])
const users = ref<User[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const status = ref<number | null>(null)

const showCreate = ref(false)
const showAssign = ref(false)
/** 编辑态：>0 表示正在编辑的 Bug id，创建/编辑共用同一表单弹窗。 */
const editingId = ref(0)
const form = reactive({ title: '', content: '', level: 2 })
const assign = reactive({ bugId: 0, toUserId: null as number | null, comment: '' })

const levelOptions = [
    { label: '1 不影响使用', value: 1 },
    { label: '2 一般', value: 2 },
    { label: '3 中等', value: 3 },
    { label: '4 严重', value: 4 },
    { label: '5 毁灭', value: 5 },
]
const statusOptions = [
    { label: '全部状态', value: 0 },
    { label: '待解决', value: 1 },
    { label: '已解决', value: 2 },
    { label: '不处理', value: 3 },
]
const statusTag: Record<number, 'warning' | 'success' | 'default'> = { 1: 'warning', 2: 'success', 3: 'default' }
const levelTag = (level: number): 'success' | 'info' | 'warning' | 'error' =>
    level >= 4 ? 'error' : level === 3 ? 'warning' : level === 2 ? 'info' : 'success'

const userOptions = computed(() => users.value.map((u) => ({ label: u.nick_name || u.name, value: u.id })))
const nameOf = (id: number): string => (id ? users.value.find((u) => u.id === id)?.nick_name || `#${id}` : '—')
const canCreate = computed(() => project.can(2))
const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await listBugs({
            project_id: projectId.value,
            status: status.value ?? undefined,
            page_num: pageNum.value,
            page_size: pageSize.value,
        })
        if (res.code === ErrCode.Success && res.data) {
            rows.value = res.data.items
            total.value = res.data.total
        }
    } finally {
        loading.value = false
    }
}

function reload(num = 1): void {
    pageNum.value = num
    void load()
}

function openCreate(): void {
    editingId.value = 0
    Object.assign(form, { title: '', content: '', level: 2 })
    showCreate.value = true
}

function openEdit(bug: Bug): void {
    editingId.value = bug.id
    Object.assign(form, { title: bug.title, content: bug.content, level: bug.level })
    showCreate.value = true
}

async function submit(): Promise<void> {
    if (!form.title.trim() || !form.content.trim()) {
        message.warning('标题与内容必填')
        return
    }
    const isEdit = editingId.value > 0
    const res = isEdit
        ? await updateBug({ id: editingId.value, ...form })
        : await createBug({ ...form, project_id: projectId.value })
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || (isEdit ? '保存失败' : '提交失败'))
        return
    }
    message.success(isEdit ? '已保存' : '已新建')
    showCreate.value = false
    editingId.value = 0
    Object.assign(form, { title: '', content: '', level: 2 })
    await load()
}

function openAssign(bug: Bug): void {
    assign.bugId = bug.id
    assign.toUserId = null
    assign.comment = ''
    showAssign.value = true
}

const showHistory = ref(false)
const history = ref<BugAssign[]>([])
const historyTitle = ref('')

/** 查看某条 Bug 的指派流转记录。 */
async function openHistory(bug: Bug): Promise<void> {
    historyTitle.value = bug.title
    const res = await listBugAssign(bug.id)
    history.value = res.code === ErrCode.Success && res.data ? res.data : []
    showHistory.value = true
}

async function submitAssign(): Promise<void> {
    if (!assign.toUserId) {
        message.warning('请选择处理人')
        return
    }
    const res = await assignBug({ bug_id: assign.bugId, to_user_id: assign.toUserId, comment: assign.comment })
    message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '已指派' : res.msg)
    if (res.code === ErrCode.Success) showAssign.value = false
    await load()
}

async function changeStatus(bug: Bug, value: number): Promise<void> {
    const res = await updateBug({ id: bug.id, status: value })
    if (res.code !== ErrCode.Success) message.error(res.msg || '更新失败')
    await load()
}

/** 置顶/取消置顶：仅提交 is_top，避免覆盖其它字段。 */
async function toggleTop(bug: Bug): Promise<void> {
    const res = await updateBug({ id: bug.id, is_top: bug.is_top ? 0 : 1 })
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || '操作失败')
        return
    }
    message.success(bug.is_top ? '已取消置顶' : '已置顶')
    await load()
}

function remove(bug: Bug): void {
    dialog.warning({
        title: '删除备忘',
        content: `确认删除「${bug.title}」？`,
        positiveText: '删除',
        negativeText: '取消',
        onPositiveClick: async () => {
            const res = await deleteBug(bug.id)
            if (res.code !== ErrCode.Success) message.error(res.msg || '删除失败')
            await load()
        },
    })
}

onMounted(async () => {
    const u = await listUsers({ page_size: 200 })
    if (u.code === ErrCode.Success && u.data) users.value = u.data.items
    await load()
})
</script>

<template>
    <div class="page">
        <div class="page-hd">
            <div>
                <h1 class="page-title">备忘清单</h1>
                <p class="page-sub">共 {{ total }} 条备忘 · 置顶优先</p>
            </div>
            <n-button v-if="canCreate" type="primary" @click="openCreate">新建备忘</n-button>
        </div>

        <n-card :bordered="true" class="table-card" content-style="padding: 0">
            <div class="filter">
                <n-select :value="status" :options="statusOptions" placeholder="状态" class="status-select"
                    @update:value="(v: number | null) => { status = v && v > 0 ? v : null; reload(1) }" />
            </div>
            <n-spin :show="loading">
                <n-empty v-if="!rows.length && !loading" class="empty-block" description="暂无备忘" />
                <n-table v-else :single-line="false" size="medium">
                    <thead>
                        <tr>
                            <th style="width: 22%">标题</th>
                            <th style="width: 110px">等级</th>
                            <th style="width: 120px">状态</th>
                            <th style="width: 110px">处理人</th>
                            <th>备注</th>
                            <th style="width: 160px">创建时间</th>
                            <th style="width: 160px">操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="b in rows" :key="b.id" :class="{ 'row--pinned': b.is_top === 1 }">
                            <td class="ellipsis" :title="b.title">
                                <n-tag v-if="b.is_top === 1" size="tiny" type="warning" :bordered="false" class="pin">置顶</n-tag>
                                {{ b.title }}
                            </td>
                            <td><n-tag :type="levelTag(b.level)" size="small" :bordered="false">{{ b.level }} 级</n-tag>
                            </td>
                            <td>
                                <n-select :value="b.status" :options="statusOptions.slice(1)" size="tiny"
                                    :disabled="!canUpdate" @update:value="changeStatus(b, $event)" />
                            </td>
                            <td>
                                <n-tag v-if="b.to_user_id" size="small" :type="statusTag[b.status] ?? 'default'"
                                    :bordered="false">
                                    {{ nameOf(b.to_user_id) }}
                                </n-tag>
                                <span v-else class="muted">未指派</span>
                            </td>
                            <td class="muted ellipsis" :title="b.comment">{{ b.comment || '—' }}</td>
                            <td class="muted nowrap">{{ fmtTime(b.create_time) }}</td>
                            <td>
                                <div class="row-actions">
                                    <n-button v-if="canUpdate" text size="small" @click="toggleTop(b)">
                                        {{ b.is_top === 1 ? '取消置顶' : '置顶' }}
                                    </n-button>
                                    <n-button v-if="canUpdate" text size="small" @click="openEdit(b)">编辑</n-button>
                                    <n-button v-if="canUpdate" text size="small" @click="openAssign(b)">指派</n-button>
                                    <n-button text size="small" @click="openHistory(b)">记录</n-button>
                                    <n-button v-if="canDelete" text type="error" size="small"
                                        @click="remove(b)">删除</n-button>
                                </div>
                            </td>
                        </tr>
                    </tbody>
                </n-table>
            </n-spin>
            <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="reload" />
        </n-card>

        <n-modal v-model:show="showCreate" preset="card" :title="editingId > 0 ? '编辑备忘' : '新建备忘'" style="width: 460px">
            <n-form label-placement="left" label-width="56">
                <n-form-item label="标题"><n-input v-model:value="form.title" /></n-form-item>
                <n-form-item label="内容"><n-input v-model:value="form.content" type="textarea" :rows="5" /></n-form-item>
                <n-form-item label="等级"><n-select v-model:value="form.level" :options="levelOptions" /></n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="showCreate = false">取消</n-button>
                    <n-button type="primary" size="small" @click="submit">保存</n-button>
                </div>
            </template>
        </n-modal>

        <n-modal v-model:show="showAssign" preset="card" title="指派处理人" style="width: 420px">
            <n-form label-placement="left" label-width="64">
                <n-form-item label="处理人">
                    <n-select v-model:value="assign.toUserId" :options="userOptions" filterable />
                </n-form-item>
                <n-form-item label="备注">
                    <n-input v-model:value="assign.comment" type="textarea" :rows="3" />
                </n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="showAssign = false">取消</n-button>
                    <n-button type="primary" size="small" @click="submitAssign">确定</n-button>
                </div>
            </template>
        </n-modal>

        <n-modal v-model:show="showHistory" preset="card" :title="`流转记录 · ${historyTitle}`" style="width: 560px">
            <n-empty v-if="!history.length" class="empty-block" description="暂无指派记录" />
            <n-table v-else :single-line="false" size="small">
                <thead>
                    <tr>
                        <th style="width: 160px">时间</th>
                        <th>从</th>
                        <th>到</th>
                        <th>备注</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="h in history" :key="h.id">
                        <td class="muted nowrap">{{ fmtTime(h.create_time) }}</td>
                        <td>{{ nameOf(h.from_user_id) }}</td>
                        <td>{{ nameOf(h.to_user_id) }}</td>
                        <td class="muted">{{ h.comment || '—' }}</td>
                    </tr>
                </tbody>
            </n-table>
        </n-modal>
    </div>
</template>

<style scoped>
.filter {
    padding: 12px 16px;
    border-bottom: 1px solid var(--app-border);
}

.status-select {
    width: 150px;
}

.pin {
    margin-right: 6px;
}

.row--pinned td {
    background: #fffbf2;
}

.foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}
</style>
