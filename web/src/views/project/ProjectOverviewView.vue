<script setup lang="ts">
// 项目概况：基本信息栅格 + 成员管理 + 右侧项目动态（对应旧版 detail 页）。
import { listUsers } from '@/api/auth'
import { listApis } from '@/api/content'
import { listLogs } from '@/api/misc'
import { addMember, listMembers, quitProject, setLeader, setPermission, updateProject } from '@/api/project'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import type { MemberView, OpLog, User } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { EllipsisVerticalOutline } from '@vicons/ionicons5'
import type { DropdownOption } from 'naive-ui'
import {
    NAvatar, NButton, NCard, NDropdown, NEmpty, NForm, NFormItem, NIcon, NInput, NModal, NSelect, NSpin, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const message = useMessage()
const userStore = useUserStore()
const project = useProjectStore()

const projectId = useNumParam('projectId')

const members = ref<MemberView[]>([])
const users = ref<User[]>([])
const logs = ref<OpLog[]>([])
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

const actionLabel: Record<string, string> = {
    create: '新增',
    update: '更新',
    delete: '删除',
    restore: '恢复',
    import: '导入',
    export: '导出',
}

const nameOf = computed(() => (id: number): string => {
    const m = members.value.find((x) => x.user_id === id)
    if (m) return m.nick_name || m.name
    const u = users.value.find((x) => x.id === id)
    return u?.nick_name || u?.name || `用户 #${id}`
})

/** 动态文案：动作与内容可能重复（如「导入」+「导入 OpenAPI…」），此处去重拼接。 */
function feedText(l: OpLog): string {
    const act = actionLabel[l.action] ?? l.action
    const content = l.content?.trim() ?? ''
    if (!content) return act
    if (content.startsWith(act)) return content
    return `${act} ${content}`
}

const memberMenu = (m: MemberView): DropdownOption[] => [
    { label: '设为负责人', key: 'leader', disabled: m.is_leader === 1 },
    { label: '权限：只读', key: 'p1', disabled: m.is_leader === 1 },
    { label: '权限：读写', key: 'p15', disabled: m.is_leader === 1 },
]

async function load(): Promise<void> {
    loading.value = true
    try {
        await project.load(projectId.value)
        const [m, u, l, a] = await Promise.all([
            listMembers(projectId.value),
            listUsers({ page_size: 200 }),
            listLogs({ project_id: projectId.value, page_size: 20 }),
            listApis({ project_id: projectId.value, page_size: 1 }),
        ])
        if (m.code === ErrCode.Success && m.data) members.value = m.data
        if (u.code === ErrCode.Success && u.data) users.value = u.data.items
        if (l.code === ErrCode.Success && l.data) logs.value = l.data.items
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

onMounted(load)
</script>

<template>
    <n-spin :show="loading">
        <div class="overview">
            <div class="ov-main">
                <n-card :bordered="true">
                    <div class="hd">
                        <div>
                            <h1 class="title">{{ project.title || '—' }}</h1>
                            <p class="desc">{{ project.current?.description || '暂无项目描述' }}</p>
                        </div>
                        <div class="hd-actions">
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
            </div>

            <aside class="ov-side">
                <n-card :bordered="true" title="项目动态">
                    <template #header-extra>
                        <span class="meta">最近 20 条</span>
                    </template>
                    <n-empty v-if="!logs.length" class="empty-block" description="暂无动态" />
                    <ul v-else class="feed">
                        <li v-for="l in logs" :key="l.id">
                            <div class="feed-time">{{ fmtTime(l.create_time) }}</div>
                            <div class="feed-text">
                                {{ nameOf(l.user_id) }}：{{ feedText(l) }}
                            </div>
                        </li>
                    </ul>
                </n-card>
            </aside>
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
    </n-spin>
</template>

<style scoped>
.overview {
    display: flex;
    align-items: flex-start;
    gap: 16px;
}

.ov-main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 16px;
}

.ov-side {
    flex: 0 0 340px;
    width: 340px;
}

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

.feed {
    list-style: none;
    margin: 0;
    padding: 0;
}

.feed li {
    padding: 8px 0 8px 12px;
    border-left: 2px solid var(--app-border);
    margin-left: 4px;
}

.feed-time {
    font-size: 12px;
    color: var(--app-muted);
}

.feed-text {
    margin-top: 2px;
    font-size: 13px;
}

.foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}

@media (max-width: 1200px) {
    .overview {
        flex-direction: column;
    }

    .ov-main,
    .ov-side {
        width: 100%;
        flex: 1 1 auto;
        align-self: stretch;
    }

    .info {
        grid-template-columns: repeat(2, 1fr);
    }
}
</style>
