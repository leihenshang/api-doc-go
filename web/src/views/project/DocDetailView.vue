<script setup lang="ts">
// 文档详情整页：Markdown 渲染查看 + 就地编辑（含图床上传）、点赞、删除。
import { deleteDoc, getDoc, likeDoc, updateDoc } from '@/api/content'
import { uploadImg } from '@/api/misc'
import { useNumParam } from '@/composables/params'
import { useUserNames } from '@/composables/userNames'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { DocItem } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { ArrowBackOutline } from '@vicons/ionicons5'
import { MdEditor, MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import 'md-editor-v3/lib/style.css'
import { NButton, NCard, NForm, NFormItem, NIcon, NInput, NSpin, useDialog, useMessage } from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const { ensure: ensureUsers, nameOf } = useUserNames()

/** embedded：由列表页（?doc=<id>）内嵌渲染，返回键改为关闭详情；docId 优先取 props。 */
const props = defineProps<{ docId?: number; embedded?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'changed'): void }>()

const projectId = useNumParam('projectId')
const docId = computed(() => props.docId ?? (Number(route.params.docId) || 0))

const doc = ref<DocItem | null>(null)
const loading = ref(false)
const saving = ref(false)
const editing = ref(route.query.edit === '1')
const form = reactive({ title: '', content: '' })

const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await getDoc(docId.value)
        if (res.code !== ErrCode.Success || !res.data) {
            message.error(res.msg || '文档加载失败')
            void back()
            return
        }
        doc.value = res.data
        Object.assign(form, { title: res.data.title, content: res.data.content })
    } finally {
        loading.value = false
    }
}

function back(): void {
  if (props.embedded) {
    emit('close')
    return
  }
  void router.push({ name: 'projectDocs', params: { projectId: projectId.value } })
}

/** 退出编辑后清掉地址栏 edit 标记，避免刷新又回到编辑态。 */
function clearEditQuery(): void {
  if (!route.query.edit) return
  if (props.embedded) {
    void router.replace({ name: 'projectDocs', params: { projectId: projectId.value }, query: { doc: String(docId.value) } })
    return
  }
  void router.replace({ name: 'docDetail', params: { projectId: projectId.value, docId: docId.value } })
}

async function save(): Promise<void> {
    if (!form.title.trim()) {
        message.warning('请输入标题')
        return
    }
    saving.value = true
    try {
        const res = await updateDoc({ id: docId.value, title: form.title.trim(), content: form.content })
        if (res.code !== ErrCode.Success) {
            message.error(res.msg || '保存失败')
            return
        }
        message.success('已保存')
        editing.value = false
        clearEditQuery()
        emit('changed')
        doc.value = res.data
    } finally {
        saving.value = false
    }
}

function cancel(): void {
    if (doc.value) Object.assign(form, { title: doc.value.title, content: doc.value.content })
    editing.value = false
    clearEditQuery()
}

// 编辑器图片上传钩子：走 /file/upload，回填 /uploads 静态路径。
async function onUploadImg(files: File[], callback: (urls: string[]) => void): Promise<void> {
    const urls: string[] = []
    for (const f of files) {
        const res = await uploadImg(f)
        if (res.code === ErrCode.Success && res.data) urls.push(res.data.url)
        else message.error(`图片 ${f.name} 上传失败`)
    }
    callback(urls)
}

async function like(): Promise<void> {
    const res = await likeDoc(docId.value)
    if (res.code !== ErrCode.Success) message.error(res.msg || '点赞失败')
    await load()
}

/** 置顶/取消置顶：后端 UpdateDoc 整体覆盖 content/group_id，须携带全量字段。 */
async function toggleTop(): Promise<void> {
    if (!doc.value) return
    const next = doc.value.is_top === 1 ? 0 : 1
    const res = await updateDoc({
        id: docId.value,
        title: doc.value.title,
        content: doc.value.content,
        group_id: doc.value.group_id,
        is_top: next,
    })
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || '操作失败')
        return
    }
    message.success(next === 1 ? '已置顶' : '已取消置顶')
    emit('changed')
    await load()
}

function remove(): void {
    dialog.warning({
        title: '删除文档',
        content: `确认删除文档「${doc.value?.title ?? ''}」？删除后可在回收站恢复。`,
        positiveText: '删除',
        negativeText: '取消',
        onPositiveClick: async () => {
            const res = await deleteDoc(docId.value)
            if (res.code !== ErrCode.Success) message.error(res.msg || '删除失败')
            emit('changed')
            back()
        },
    })
}

onMounted(() => {
    void ensureUsers()
    void load()
})
</script>

<template>
    <n-spin :show="loading">
        <div class="page">
            <div class="toolbar">
                <n-button size="small" @click="back">
                    <template #icon><n-icon :component="ArrowBackOutline" /></template>
                    {{ embedded ? '关闭' : '文档列表' }}
                </n-button>
                <div class="toolbar-end">
                    <template v-if="editing">
                        <n-button size="small" @click="cancel">取消</n-button>
                        <n-button size="small" type="primary" :loading="saving" @click="save">保存</n-button>
                    </template>
                    <template v-else>
                        <n-button size="small" tertiary @click="like">👍 {{ doc?.like_count ?? 0 }}</n-button>
                        <n-button v-if="canUpdate" size="small" tertiary @click="toggleTop">
                            {{ doc?.is_top === 1 ? '取消置顶' : '置顶' }}
                        </n-button>
                        <n-button v-if="canUpdate" size="small" type="primary" @click="editing = true">修改文档</n-button>
                        <n-button v-if="canDelete" size="small" tertiary type="error" @click="remove">删除</n-button>
                    </template>
                </div>
            </div>

            <n-card :bordered="true" class="doc-card">
                <template v-if="editing">
                    <n-form label-placement="left" label-width="56">
                        <n-form-item label="标题"><n-input v-model:value="form.title" /></n-form-item>
                    </n-form>
                    <md-editor v-model="form.content" preview-theme="default" style="height: 65vh"
                        @on-upload-img="onUploadImg" />
                </template>

                <template v-else>
                    <h1 class="doc-title">{{ doc?.title || '—' }}</h1>
                    <p class="doc-meta">
                        作者:{{ doc ? nameOf(doc.user_id) : '—' }}
                        <span class="sep">·</span>{{ fmtTime(doc?.update_time) }}
                        <span class="sep">·</span>阅读次数 {{ doc?.view_count ?? 0 }}
                        <span class="sep">·</span>点赞次数 {{ doc?.like_count ?? 0 }}
                    </p>
                    <md-preview id="doc-preview" v-model:model-value="form.content" theme="light"
                        preview-theme="default" />
                </template>
            </n-card>
        </div>
    </n-spin>
</template>

<style scoped>
.doc-card {
    max-width: 1080px;
}

.doc-title {
    margin: 8px 0 0;
    text-align: center;
    font-size: 24px;
    font-weight: 600;
}

.doc-meta {
    margin: 12px 0 20px;
    text-align: center;
    font-size: 12px;
    color: var(--app-muted);
}

.sep {
    margin: 0 8px;
}
</style>
