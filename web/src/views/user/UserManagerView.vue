<script setup lang="ts">
// 用户管理：列表/检索，管理员可改昵称、邮箱、状态与角色。
import { listUsers, updateProfile } from '@/api/auth'
import TablePager from '@/components/TablePager.vue'
import { ErrCode } from '@/types/api'
import type { User } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import {
    NButton, NCard, NEmpty, NForm, NFormItem, NInput, NModal, NSelect, NSpin, NTable, NTag, useMessage,
} from 'naive-ui'
import { onMounted, reactive, ref } from 'vue'

const message = useMessage()
const rows = ref<User[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const keyword = ref('')
const show = ref(false)
const edit = reactive({ id: 0, nick_name: '', email: '', state: 1, type: 1 })

const stateOptions = [
  { label: '正常', value: 1 },
  { label: '禁用', value: 2 },
]
const typeOptions = [
  { label: '普通用户', value: 1 },
  { label: '管理员', value: 2 },
  { label: '超级管理员', value: 3 },
]

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listUsers({ keyword: keyword.value, page_num: pageNum.value, page_size: pageSize.value })
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

function openEdit(u: User): void {
  Object.assign(edit, { id: u.id, nick_name: u.nick_name, email: u.email, state: u.state, type: u.type })
  show.value = true
}

async function save(): Promise<void> {
  const res = await updateProfile(edit.id, {
    nick_name: edit.nick_name,
    email: edit.email,
    state: edit.state,
    type: edit.type,
  })
  message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '已保存' : res.msg)
  show.value = false
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-hd">
      <div>
        <h1 class="page-title">用户管理</h1>
        <p class="page-sub">共 {{ total }} 位用户</p>
      </div>
      <n-input v-model:value="keyword" placeholder="按账号 / 昵称前缀检索" clearable class="search" @keyup.enter="reload(1)"
        @clear="reload(1)" />
    </div>

    <n-card :bordered="true" class="table-card" content-style="padding: 0">
      <n-spin :show="loading">
        <n-empty v-if="!rows.length && !loading" class="empty-block" description="没有匹配的用户" />
        <n-table v-else :single-line="false" size="medium">
          <thead>
            <tr>
              <th style="width: 70px">ID</th>
              <th style="width: 16%">账号</th>
              <th style="width: 16%">昵称</th>
              <th>邮箱</th>
              <th style="width: 120px">角色</th>
              <th style="width: 90px">状态</th>
              <th style="width: 170px">注册时间</th>
              <th style="width: 80px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in rows" :key="u.id">
              <td class="muted">{{ u.id }}</td>
              <td>{{ u.name }}</td>
              <td>{{ u.nick_name || '—' }}</td>
              <td class="muted ellipsis" :title="u.email">{{ u.email || '—' }}</td>
              <td class="muted nowrap">{{typeOptions.find((o) => o.value === u.type)?.label ?? u.type}}</td>
              <td>
                <n-tag :type="u.state === 1 ? 'success' : 'error'" size="small" :bordered="false">
                  {{ u.state === 1 ? '正常' : '禁用' }}
                </n-tag>
              </td>
              <td class="muted nowrap">{{ fmtTime(u.create_time) }}</td>
              <td><n-button text type="primary" size="small" @click="openEdit(u)">编辑</n-button></td>
            </tr>
          </tbody>
        </n-table>
      </n-spin>
      <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="reload" />
    </n-card>

    <n-modal v-model:show="show" preset="card" title="编辑用户" style="width: 420px">
      <n-form label-placement="left" label-width="56">
        <n-form-item label="昵称"><n-input v-model:value="edit.nick_name" /></n-form-item>
        <n-form-item label="邮箱"><n-input v-model:value="edit.email" /></n-form-item>
        <n-form-item label="状态"><n-select v-model:value="edit.state" :options="stateOptions" /></n-form-item>
        <n-form-item label="角色"><n-select v-model:value="edit.type" :options="typeOptions" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="show = false">取消</n-button>
          <n-button type="primary" size="small" @click="save">保存</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.search {
  width: 240px;
}

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
