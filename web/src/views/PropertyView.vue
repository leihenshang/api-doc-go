<script setup lang="ts">
// 公共属性管理（管理员）：标签字典的列表/新增/删除。接口一次返回全量，前端切片分页。
import { deleteProperty, listProperties, saveProperty } from '@/api/misc'
import TablePager from '@/components/TablePager.vue'
import { ErrCode } from '@/types/api'
import { NButton, NCard, NEmpty, NForm, NFormItem, NInput, NModal, NTable, useMessage } from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'

interface Property {
  id: number
  tag: string
  tag_name: string
  description: string
}

const message = useMessage()
const rows = ref<Property[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const show = ref(false)
const form = reactive({ tag: '', tag_name: '', description: '' })

const paged = computed(() => rows.value.slice((pageNum.value - 1) * pageSize.value, pageNum.value * pageSize.value))

async function reload(): Promise<void> {
  const res = await listProperties()
  if (res.code === ErrCode.Success && res.data) {
    rows.value = res.data as unknown as Property[]
    total.value = rows.value.length
  }
}

/** 删除/保存后如当前页超界，回退到最后一页。 */
function clampPage(): void {
  const maxPage = Math.max(1, Math.ceil(rows.value.length / pageSize.value))
  if (pageNum.value > maxPage) pageNum.value = maxPage
}

async function save(): Promise<void> {
  if (!form.tag.trim() || !form.tag_name.trim()) {
    message.warning('标记与名称必填')
    return
  }
  const res = await saveProperty({ ...form })
  message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? '已保存' : res.msg)
  show.value = false
  Object.assign(form, { tag: '', tag_name: '', description: '' })
  await reload()
  clampPage()
}

async function remove(id: number): Promise<void> {
  const res = await deleteProperty(id)
  if (res.code !== ErrCode.Success) message.error(res.msg)
  await reload()
  clampPage()
}

onMounted(reload)
</script>

<template>
  <div class="page">
    <div class="page-hd">
      <div>
        <h1 class="page-title">公共属性</h1>
        <p class="page-sub">全局标签字典，供各模块下拉与标签复用；仅管理员可维护</p>
      </div>
      <n-button type="primary" @click="show = true">新增属性</n-button>
    </div>

    <n-card :bordered="true" class="table-card" content-style="padding: 0">
      <n-empty v-if="!rows.length" class="empty-block" description="暂无属性" />
      <n-table v-else :single-line="false" size="medium">
        <thead>
          <tr>
            <th style="width: 22%">标记</th>
            <th style="width: 22%">名称</th>
            <th>描述</th>
            <th style="width: 80px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in paged" :key="p.id">
            <td class="row-mono">{{ p.tag }}</td>
            <td>{{ p.tag_name }}</td>
            <td class="muted">{{ p.description || '—' }}</td>
            <td><n-button text type="error" size="small" @click="remove(p.id)">删除</n-button></td>
          </tr>
        </tbody>
      </n-table>
      <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="pageNum = $event" />
    </n-card>

    <n-modal v-model:show="show" preset="card" title="新增属性" style="width: 420px">
      <n-form label-placement="left" label-width="56">
        <n-form-item label="标记"><n-input v-model:value="form.tag" placeholder="如 api_protocol" /></n-form-item>
        <n-form-item label="名称"><n-input v-model:value="form.tag_name" /></n-form-item>
        <n-form-item label="描述"><n-input v-model:value="form.description" /></n-form-item>
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
.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
