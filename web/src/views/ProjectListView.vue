<script setup lang="ts">
// 项目列表：卡片栅格 + 分页 + 新建/编辑/删除/进入详情。
import { createProject, deleteProject, listProjects, updateProject } from '@/api/project'
import TablePager from '@/components/TablePager.vue'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { Project } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { AddOutline } from '@vicons/ionicons5'
import {
    NButton, NEmpty, NForm, NFormItem, NIcon, NInput, NModal, NSpin, NSwitch, useDialog, useMessage,
} from 'naive-ui'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const { t } = useI18n()

const rows = ref<Project[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(12)
const loading = ref(false)

const showEdit = ref(false)
const editId = ref(0)
const form = reactive({ title: '', description: '', version: '', type: '', client_write_enabled: true })

/** 复制项目 ID（供桌面客户端绑定使用）。 */
async function copyId(id: number): Promise<void> {
  await navigator.clipboard.writeText(String(id))
  message.success(t('common.copied'))
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listProjects({ page_num: pageNum.value, page_size: pageSize.value })
    if (res.code === ErrCode.Success && res.data) {
      rows.value = res.data.items
      total.value = res.data.total
    } else {
      message.error(res.msg || t('common.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function page(num: number): Promise<void> {
  pageNum.value = num
  await load()
}

function openCreate(): void {
  editId.value = 0
  Object.assign(form, { title: '', description: '', version: '', type: '', client_write_enabled: true })
  showEdit.value = true
}

function openEdit(p: Project): void {
  editId.value = p.id
  Object.assign(form, {
    title: p.title,
    description: p.description,
    version: p.version,
    type: p.type,
    client_write_enabled: p.client_write_enabled ?? true,
  })
  showEdit.value = true
}

async function submit(): Promise<void> {
  if (!form.title.trim()) {
    message.warning(t('project.nameRequired'))
    return
  }
  const res = editId.value
    ? await updateProject({ id: editId.value, ...form })
    : await createProject(form)
  if (res.code !== ErrCode.Success || !res.data) {
    message.error(res.msg || (editId.value ? t('common.saveFailed') : t('common.createFailed')))
    return
  }
  message.success(editId.value ? t('common.saved') : t('project.created'))
  if (project.id === res.data.id) project.set(res.data)
  showEdit.value = false
  await Promise.all([load(), project.loadProjects(true)])
}

function remove(p: Project): void {
  dialog.warning({
    title: t('project.deleteTitle'),
    content: t('project.deleteConfirm', { title: p.title }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = await deleteProject(p.id)
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.deleteFailed'))
        return
      }
      message.success(t('common.deleted'))
      if (project.id === p.id) project.clear()
      await Promise.all([load(), project.loadProjects(true)])
    },
  })
}

function open(p: Project): void {
  void router.push({ name: 'projectApis', params: { projectId: p.id } })
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-hd">
      <div>
        <h1 class="page-title">{{ t('project.title') }}</h1>
        <p class="page-sub">{{ t('project.sub', { n: total }) }}</p>
      </div>
      <n-button type="primary" @click="openCreate">{{ t('project.create') }}</n-button>
    </div>

    <n-spin :show="loading">
      <n-empty v-if="!rows.length && !loading" class="empty-block" :description="t('project.empty')" />
      <div v-else class="cards">
        <div v-for="p in rows" :key="p.id" class="pcard" @click="open(p)">
          <div class="pcard-hd">
            <router-link class="pcard-title ellipsis" :title="p.title"
              :to="{ name: 'projectApis', params: { projectId: p.id } }" @click.stop>
              {{ p.title }}
            </router-link>
            <div class="pcard-badges">
              <span class="badge badge--id" :title="t('project.idHint')" @click.stop="copyId(p.id)">#{{ p.id }}</span>
              <span v-if="p.type" class="badge">{{ p.type }}</span>
              <span v-if="p.version" class="badge badge--muted">{{ p.version }}</span>
            </div>
          </div>
          <p class="pcard-desc" :title="p.description">{{ p.description || t('project.noDesc') }}</p>
          <div class="pcard-ft">
            <span class="muted">{{ fmtTime(p.create_time) }}</span>
            <div class="row-actions" @click.stop>
              <n-button text type="primary" size="small" @click="open(p)">{{ t('project.detail') }}</n-button>
              <n-button text size="small" @click="openEdit(p)">{{ t('common.edit') }}</n-button>
              <n-button text type="error" size="small" @click="remove(p)">{{ t('common.delete') }}</n-button>
            </div>
          </div>
        </div>
        <!-- 新建磁贴：项目少时右侧会留大片空白，这里给一个可直接点的出口（UI-01） -->
        <button class="pcard pcard--new" type="button" data-testid="project.newTile" @click="openCreate">
          <n-icon :component="AddOutline" :size="20" />
          <span>{{ t('project.create') }}</span>
        </button>
      </div>
      <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="page" />
    </n-spin>

    <n-modal v-model:show="showEdit" preset="card" :title="editId ? t('project.editTitle') : t('project.createTitle')" style="width: 460px">
      <n-form label-placement="left" label-width="72">
        <n-form-item :label="t('project.name')">
          <n-input v-model:value="form.title" :placeholder="t('project.namePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('project.version')">
          <n-input v-model:value="form.version" :placeholder="t('project.versionPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('project.type')">
          <n-input v-model:value="form.type" :placeholder="t('project.typePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('project.description')">
          <n-input v-model:value="form.description" type="textarea" :rows="3" :placeholder="t('common.optional')" />
        </n-form-item>
        <n-form-item v-if="editId" :label="t('project.clientWrite')">
          <n-switch v-model:value="form.client_write_enabled" />
          <span class="form-hint">{{ t('project.clientWriteHint') }}</span>
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="showEdit = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" size="small" @click="submit">{{ t('common.save') }}</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

/* 整卡可点击进入项目详情 */
.pcard {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  background: #fff;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  transition: box-shadow 0.15s ease, border-color 0.15s ease;
}

.pcard--new {
  /* 与普通卡片同尺寸的虚线磁贴：居中 + 主题色，hover 时边框变实。 */
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 132px;
  border-style: dashed;
  background: transparent;
  color: var(--app-accent);
  font-size: 13px;
  cursor: pointer;
}

.pcard--new:hover {
  border-style: solid;
  background: var(--app-accent-tint);
}

.pcard:hover {
  border-color: var(--app-accent);
  box-shadow: 0 2px 10px rgba(24, 160, 88, 0.1);
}

.pcard-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.pcard-title {
  min-width: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text);
  text-decoration: none;
}

.pcard-title:hover {
  color: var(--app-accent);
}

.pcard-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
}

.badge {
  padding: 1px 8px;
  border: 1px solid var(--app-accent);
  border-radius: 999px;
  background: rgba(24, 160, 88, 0.08);
  color: var(--app-accent);
  font-size: 11px;
  line-height: 16px;
  white-space: nowrap;
}

.badge--muted {
  border-color: var(--app-border);
  background: #f5f6f8;
  color: var(--app-muted);
}

.badge--id {
  cursor: pointer;
  font-family: var(--app-font-mono, monospace);
}

.form-hint {
  font-size: 12px;
  color: var(--app-muted);
  margin-left: 8px;
}

.pcard-desc {
  margin: 0;
  min-height: 20px;
  font-size: 13px;
  color: var(--app-muted);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.pcard-ft {
  margin-top: auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--app-border);
  font-size: 12px;
}

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
