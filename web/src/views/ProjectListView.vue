<script setup lang="ts">
// 项目列表：表格展示 + 分页 + 新建/编辑/删除/进入详情。
import { createProject, deleteProject, listProjects, updateProject } from '@/api/project'
import TablePager from '@/components/TablePager.vue'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { Project } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import {
    NButton, NCard, NEmpty, NForm, NFormItem, NInput, NModal, NSpin, NTable, useDialog, useMessage,
} from 'naive-ui'
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()

const rows = ref<Project[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)

const showEdit = ref(false)
const editId = ref(0)
const form = reactive({ title: '', description: '', version: '', type: '' })

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listProjects({ page_num: pageNum.value, page_size: pageSize.value })
    if (res.code === ErrCode.Success && res.data) {
      rows.value = res.data.items
      total.value = res.data.total
    } else {
      message.error(res.msg || '加载失败')
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
  Object.assign(form, { title: '', description: '', version: '', type: '' })
  showEdit.value = true
}

function openEdit(p: Project): void {
  editId.value = p.id
  Object.assign(form, { title: p.title, description: p.description, version: p.version, type: p.type })
  showEdit.value = true
}

async function submit(): Promise<void> {
  if (!form.title.trim()) {
    message.warning('请输入项目名称')
    return
  }
  const res = editId.value
    ? await updateProject({ id: editId.value, ...form })
    : await createProject(form)
  if (res.code !== ErrCode.Success || !res.data) {
    message.error(res.msg || (editId.value ? '保存失败' : '创建失败'))
    return
  }
  message.success(editId.value ? '已保存' : '项目已创建')
  if (project.id === res.data.id) project.set(res.data)
  showEdit.value = false
  await load()
}

function remove(p: Project): void {
  dialog.warning({
    title: '删除项目',
    content: `确认删除项目「${p.title}」？删除后可在数据库层恢复。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const res = await deleteProject(p.id)
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || '删除失败')
        return
      }
      message.success('已删除')
      if (project.id === p.id) project.clear()
      await load()
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
        <h1 class="page-title">项目列表</h1>
        <p class="page-sub">共 {{ total }} 个项目</p>
      </div>
      <n-button type="primary" @click="openCreate">新增项目</n-button>
    </div>

    <n-card :bordered="true" class="table-card" content-style="padding: 0">
      <n-spin :show="loading">
        <n-empty v-if="!rows.length && !loading" class="empty-block" description="还没有项目，点击右上角新增" />
        <n-table v-else :single-line="false" size="medium">
          <thead>
            <tr>
              <th style="width: 22%">项目名称</th>
              <th style="width: 10%">版本号</th>
              <th style="width: 10%">类型</th>
              <th>描述</th>
              <th style="width: 170px">创建时间</th>
              <th style="width: 170px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in rows" :key="p.id">
              <td>
                <router-link class="link" :to="{ name: 'projectApis', params: { projectId: p.id } }">
                  {{ p.title }}
                </router-link>
              </td>
              <td>{{ p.version || '—' }}</td>
              <td>{{ p.type || '—' }}</td>
              <td class="ellipsis muted" :title="p.description">{{ p.description || '—' }}</td>
              <td class="muted nowrap">{{ fmtTime(p.create_time) }}</td>
              <td>
                <div class="row-actions">
                  <n-button text type="primary" size="small" @click="open(p)">详情</n-button>
                  <n-button text size="small" @click="openEdit(p)">编辑</n-button>
                  <n-button text type="error" size="small" @click="remove(p)">删除</n-button>
                </div>
              </td>
            </tr>
          </tbody>
        </n-table>
      </n-spin>
      <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="page" />
    </n-card>

    <n-modal v-model:show="showEdit" preset="card" :title="editId ? '编辑项目' : '新增项目'" style="width: 460px">
      <n-form label-placement="left" label-width="72">
        <n-form-item label="名称">
          <n-input v-model:value="form.title" placeholder="项目唯一名称" />
        </n-form-item>
        <n-form-item label="版本号">
          <n-input v-model:value="form.version" placeholder="如 v1.0" />
        </n-form-item>
        <n-form-item label="类型">
          <n-input v-model:value="form.type" placeholder="如 web / pc / app" />
        </n-form-item>
        <n-form-item label="描述">
          <n-input v-model:value="form.description" type="textarea" :rows="3" placeholder="可选" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="showEdit = false">取消</n-button>
          <n-button type="primary" size="small" @click="submit">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

td .link {
  font-weight: 500;
}
</style>
