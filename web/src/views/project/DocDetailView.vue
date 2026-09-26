<script setup lang="ts">
// 文档详情整页：Markdown 渲染查看 + 就地编辑（含图床上传）、点赞、删除。
import { deleteDoc, getDoc, likeDoc, updateDoc } from '@/api/content'
import { uploadImg } from '@/api/misc'
import { useMdUpload } from '@/composables/md-upload'
import { useNumParam } from '@/composables/params'
import { useUserNames } from '@/composables/userNames'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { DocItem } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { DEFAULT_DOC_ICON, isImageIcon } from '@/utils/icon'
import { ArrowBackOutline } from '@vicons/ionicons5'
import { MdEditor, MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import 'md-editor-v3/lib/style.css'
import {
    NButton, NCard, NForm, NFormItem, NIcon, NInput, NSpin, NUpload, useDialog, useMessage,
    type UploadFileInfo,
} from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { t } = useI18n()
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
/** icon：文档图标（emoji 或上传后的图片 URL）。 */
const form = reactive({ title: '', content: '', icon: '' })
const iconUploading = ref(false)

/** 上传文档图标：复用图床接口 /file/upload，成功后回填图片 URL。 */
async function onIconFile(list: UploadFileInfo[]): Promise<void> {
    const file = list[0]?.file
    if (!file) return
    iconUploading.value = true
    try {
        const res = await uploadImg(file)
        if (res.code !== ErrCode.Success || !res.data) {
            message.error(res.msg || t('docDetail.iconUploadFailed'))
            return
        }
        form.icon = res.data.url
        message.success(t('docDetail.iconUploaded'))
    } finally {
        iconUploading.value = false
    }
}

const canUpdate = computed(() => project.can(4))
const canDelete = computed(() => project.can(8))

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await getDoc(docId.value)
        if (res.code !== ErrCode.Success || !res.data) {
            message.error(res.msg || t('docDetail.loadFailed'))
            void back()
            return
        }
        doc.value = res.data
        Object.assign(form, { title: res.data.title, content: res.data.content, icon: res.data.icon ?? '' })
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
        message.warning(t('doc.titleRequired'))
        return
    }
    saving.value = true
    try {
        const res = await updateDoc({
            id: docId.value,
            title: form.title.trim(),
            content: form.content,
            icon: form.icon.trim(),
            // 乐观锁：他人已保存时服务端会拒绝覆盖。
            expected_update_time: doc.value?.update_time ?? '',
        })
        if (res.code !== ErrCode.Success) {
            message.error(res.msg || t('common.saveFailed'))
            if ((res.msg || '').includes('已被他人修改')) {
                dialog.warning({
                    title: t('detail.conflictTitle'),
                    content: t('detail.conflictContent', { msg: res.msg }),
                    positiveText: t('detail.conflictReload'),
                    negativeText: t('detail.conflictLater'),
                    onPositiveClick: async () => {
                        await load()
                        editing.value = false
                        clearEditQuery()
                    },
                })
            }
            return
        }
        message.success(t('common.saved'))
        editing.value = false
        clearEditQuery()
        emit('changed')
        doc.value = res.data
    } finally {
        saving.value = false
    }
}

function cancel(): void {
    if (doc.value) Object.assign(form, { title: doc.value.title, content: doc.value.content, icon: doc.value.icon ?? '' })
    editing.value = false
    clearEditQuery()
}

const onUploadImg = useMdUpload()

async function like(): Promise<void> {
    const res = await likeDoc(docId.value)
    if (res.code !== ErrCode.Success) message.error(res.msg || t('docDetail.likeFailed'))
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
        icon: doc.value.icon ?? '',
        group_id: doc.value.group_id,
        is_top: next,
        // 置顶同样是整体覆盖：带上载入时间，避免用旧内容覆盖他人修改。
        expected_update_time: doc.value.update_time,
    })
    if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.opFailed'))
        if ((res.msg || '').includes('已被他人修改')) await load()
        return
    }
    message.success(next === 1 ? t('docDetail.topped') : t('docDetail.untopped'))
    emit('changed')
    await load()
}

function remove(): void {
    dialog.warning({
        title: t('doc.deleteTitle'),
        content: t('doc.deleteConfirm', { title: doc.value?.title ?? '' }),
        positiveText: t('common.delete'),
        negativeText: t('common.cancel'),
        onPositiveClick: async () => {
            const res = await deleteDoc(docId.value)
            if (res.code !== ErrCode.Success) message.error(res.msg || t('common.deleteFailed'))
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
                    {{ embedded ? t('common.close') : t('doc.listTitle') }}
                </n-button>
                <div class="toolbar-end">
                    <template v-if="editing">
                        <n-button size="small" @click="cancel">{{ t('common.cancel') }}</n-button>
                        <n-button size="small" type="primary" :loading="saving" @click="save">{{ t('common.save') }}</n-button>
                    </template>
                    <template v-else>
                        <n-button size="small" tertiary @click="like">👍 {{ doc?.like_count ?? 0 }}</n-button>
                        <n-button v-if="canUpdate" size="small" tertiary @click="toggleTop">
                            {{ doc?.is_top === 1 ? t('docDetail.untop') : t('docDetail.top') }}
                        </n-button>
                        <n-button v-if="canUpdate" size="small" type="primary" @click="editing = true">{{ t('docDetail.edit') }}</n-button>
                        <n-button v-if="canDelete" size="small" tertiary type="error" @click="remove">{{ t('common.delete') }}</n-button>
                    </template>
                </div>
            </div>

            <n-card :bordered="true" class="doc-card">
                <template v-if="editing">
                    <n-form label-placement="left" label-width="56">
                        <n-form-item :label="t('doc.titleField')"><n-input v-model:value="form.title" /></n-form-item>
                        <n-form-item :label="t('docDetail.icon')">
                            <div class="icon-row">
                                <img v-if="isImageIcon(form.icon)" class="icon-preview-img" :src="form.icon" alt="" />
                                <span v-else class="icon-preview">{{ form.icon || DEFAULT_DOC_ICON }}</span>
                                <n-input v-model:value="form.icon" :placeholder="t('docDetail.iconPlaceholder')" class="icon-input" />
                                <n-upload :max="1" :default-upload="false" accept="image/*" :show-file-list="false"
                                    @update:file-list="onIconFile">
                                    <n-button size="small" :loading="iconUploading">{{ t('docDetail.uploadImage') }}</n-button>
                                </n-upload>
                                <n-button v-if="form.icon" size="small" text type="error" @click="form.icon = ''">{{ t('docDetail.clearIcon') }}</n-button>
                            </div>
                        </n-form-item>
                    </n-form>
                    <md-editor v-model="form.content" preview-theme="default" style="height: 65vh"
                        @on-upload-img="onUploadImg" />
                </template>

                <template v-else>
                    <h1 class="doc-title">
                        <img v-if="isImageIcon(doc?.icon)" class="doc-icon-img" :src="doc?.icon ?? ''" alt="" />
                        <span v-else class="doc-icon">{{ doc?.icon || DEFAULT_DOC_ICON }}</span>{{ doc?.title || '—' }}
                    </h1>
                    <p class="doc-meta">
                        {{ t('docDetail.author') }}:{{ doc ? nameOf(doc.user_id) : '—' }}
                        <span class="sep">·</span>{{ fmtTime(doc?.update_time) }}
                        <span class="sep">·</span>{{ t('docDetail.views', { n: doc?.view_count ?? 0 }) }}
                        <span class="sep">·</span>{{ t('docDetail.likes', { n: doc?.like_count ?? 0 }) }}
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

/* 图标编辑行：预览 + 输入 + 上传 */
.icon-row {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
}

.icon-preview {
    flex: 0 0 auto;
    font-size: 20px;
    line-height: 1;
}

.icon-input {
    flex: 1 1 auto;
    min-width: 0;
}

/* 文档标题前的图标（emoji 或图片） */
.doc-icon {
    margin-right: 8px;
    font-size: 22px;
    vertical-align: -2px;
}

.doc-icon-img {
    width: 22px;
    height: 22px;
    margin-right: 8px;
    border-radius: 4px;
    object-fit: cover;
    vertical-align: -4px;
}

.icon-preview-img {
    width: 22px;
    height: 22px;
    border-radius: 4px;
    object-fit: cover;
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
