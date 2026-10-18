<script setup lang="ts">
// 分享链接面板：为项目生成**免登录只读**入口（可设有效期、可随时失效）。
// 访问地址为前端路由 `/share/<token>`，打开即进入只读文档站视图。
import { createShareLink, deleteShareLink, listShareLinks } from '@/api/share'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { ShareLink } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import {
    NButton, NCard, NForm, NFormItem, NInput, NInputNumber, NModal, NSpin, NTable,
    useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'

const props = defineProps<{ projectId: number }>()

const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()

const rows = ref<ShareLink[]>([])
const loading = ref(false)
const show = ref(false)
const saving = ref(false)
const form = reactive({ remark: '', expireDays: 7 })

const canManage = computed(() => project.can(4))

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await listShareLinks(props.projectId)
        rows.value = res.code === ErrCode.Success && res.data ? res.data : []
    } finally {
        loading.value = false
    }
}

/** 分享链接的完整地址。 */
function shareURL(link: ShareLink): string {
    return `${location.origin}/share/${link.token}`
}

function expired(link: ShareLink): boolean {
    return Boolean(link.expire_time && new Date(link.expire_time).getTime() < Date.now())
}

/** 有效期文案：永久 / 具体时间（已过期标红提示）。 */
function expireText(link: ShareLink): string {
    if (!link.expire_time) return '永久有效'
    return `${fmtTime(link.expire_time)}${expired(link) ? '（已过期）' : ''}`
}

async function copy(link: ShareLink): Promise<void> {
    const url = shareURL(link)
    try {
        await navigator.clipboard.writeText(url)
        message.success('分享链接已复制')
    } catch {
        message.warning(url)
    }
}

async function submit(): Promise<void> {
    saving.value = true
    try {
        const res = await createShareLink({
            project_id: props.projectId,
            remark: form.remark.trim(),
            expire_days: form.expireDays,
        })
        if (res.code !== ErrCode.Success || !res.data) {
            message.error(res.msg || '生成失败')
            return
        }
        message.success('分享链接已生成')
        show.value = false
        Object.assign(form, { remark: '', expireDays: 7 })
        await load()
        await copy(res.data)
    } finally {
        saving.value = false
    }
}

function remove(link: ShareLink): void {
    dialog.warning({
        title: '失效分享链接',
        content: '失效后该链接立即无法访问（已发出的链接会提示「分享链接不存在或已失效」），确认？',
        positiveText: '失效',
        negativeText: '取消',
        onPositiveClick: async () => {
            const res = await deleteShareLink(link.id)
            if (res.code !== ErrCode.Success) {
                message.error(res.msg || '操作失败')
                return false
            }
            message.success('已失效')
            await load()
        },
    })
}

onMounted(load)
</script>

<template>
    <n-card :bordered="true" title="分享链接（只读）">
        <template #header-extra>
            <n-button v-if="canManage" size="small" @click="show = true">生成分享链接</n-button>
        </template>
        <p class="meta share-tip">
            拿到链接的人<b>无需登录</b>即可查看本项目的接口与文档（只读）；令牌可随时失效，也可设置有效期。
        </p>
        <n-spin :show="loading">
            <div v-if="!rows.length && !loading" class="share-empty">
                <span class="meta">尚未生成分享链接</span>
                <n-button v-if="canManage" text type="primary" size="small" data-testid="share.emptyCreate"
                    @click="show = true">生成一条</n-button>
            </div>
            <n-table v-else :single-line="false" size="small">
                <thead>
                    <tr>
                        <th style="width: 22%">备注</th>
                        <th>链接</th>
                        <th style="width: 150px">有效期</th>
                        <th style="width: 150px">创建时间</th>
                        <th style="width: 120px">操作</th>
                    </tr>
                </thead>
                <tbody>
                    <tr v-for="s in rows" :key="s.id">
                        <td>{{ s.remark || '—' }}</td>
                        <td class="row-mono ellipsis" :title="shareURL(s)">{{ shareURL(s) }}</td>
                        <td :class="expired(s) ? 'err' : 'muted'">{{ expireText(s) }}</td>
                        <td class="muted nowrap">{{ fmtTime(s.create_time) }}</td>
                        <td>
                            <div class="row-actions">
                                <n-button text size="small" @click="copy(s)">复制</n-button>
                                <n-button v-if="canManage" text type="error" size="small" @click="remove(s)">失效</n-button>
                            </div>
                        </td>
                    </tr>
                </tbody>
            </n-table>
        </n-spin>

        <n-modal v-model:show="show" preset="card" title="生成分享链接" style="width: 440px">
            <n-form label-placement="left" label-width="82">
                <n-form-item label="备注">
                    <n-input v-model:value="form.remark" placeholder="如：给前端同学联调" />
                </n-form-item>
                <n-form-item label="有效期">
                    <n-input-number v-model:value="form.expireDays" :min="0" :max="3650" />
                    <span class="meta expire-tip">天（0 = 永久有效）</span>
                </n-form-item>
            </n-form>
            <template #footer>
                <div class="foot">
                    <n-button size="small" @click="show = false">取消</n-button>
                    <n-button type="primary" size="small" :loading="saving" @click="submit">生成</n-button>
                </div>
            </template>
        </n-modal>
    </n-card>
</template>

<style scoped>
/* 空态压成一行：n-empty 自带大段留白，这里只有一个可选项，不值得占 150px（UI-06）。 */
.share-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 14px 0;
    border: 1px dashed var(--app-border);
    border-radius: 4px;
}

.share-tip {
    margin: 0 0 12px;
}

.expire-tip {
    margin-left: 8px;
}

.err {
    color: #d03050;
}

.foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
}
</style>
