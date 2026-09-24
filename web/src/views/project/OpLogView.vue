<script setup lang="ts">
// 操作日志：按项目分页查看操作轨迹，可按对象类型过滤。
import { listLogs } from '@/api/misc'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useUserNames } from '@/composables/userNames'
import { ErrCode } from '@/types/api'
import type { OpLog } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { NCard, NEmpty, NSelect, NSpin, NTable } from 'naive-ui'
import { onMounted, ref } from 'vue'

const projectId = useNumParam('projectId')
const { ensure: ensureUsers, nameOf } = useUserNames()

const rows = ref<OpLog[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const type = ref<number | null>(null)

const typeOptions = [
    { label: '全部类型', value: 0 },
    { label: '项目', value: 1 },
    { label: '分组', value: 2 },
    { label: '接口', value: 3 },
    { label: '用户', value: 4 },
    { label: '文档', value: 5 },
]
const actionLabel: Record<string, string> = {
    create: '新增',
    update: '更新',
    delete: '删除',
    restore: '恢复',
    import: '导入',
    export: '导出',
}

async function load(): Promise<void> {
    loading.value = true
    try {
        const res = await listLogs({
            project_id: projectId.value,
            type: type.value ?? 0,
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

onMounted(() => {
    void ensureUsers()
    void load()
})
</script>

<template>
    <div class="page">
        <div class="page-hd">
            <div>
                <h1 class="page-title">操作日志</h1>
                <p class="page-sub">共 {{ total }} 条记录，按平台策略保留 70 天</p>
            </div>
            <n-select :value="type" :options="typeOptions" placeholder="对象类型" class="type-select"
                @update:value="(v: number | null) => { type = v && v > 0 ? v : null; reload(1) }" />
        </div>

        <n-card :bordered="true" class="table-card" content-style="padding: 0">
            <n-spin :show="loading">
                <n-empty v-if="!rows.length && !loading" class="empty-block" description="暂无日志" />
                <n-table v-else :single-line="false" size="medium">
                    <thead>
                        <tr>
                            <th style="width: 170px">时间</th>
                            <th style="width: 140px">操作人</th>
                            <th style="width: 90px">动作</th>
                            <th style="width: 120px">对象类型</th>
                            <th>内容</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr v-for="l in rows" :key="l.id">
                            <td class="muted">{{ fmtTime(l.create_time) }}</td>
                            <td>{{ nameOf(l.user_id) }}</td>
                            <td>{{ actionLabel[l.action] ?? l.action }}</td>
                            <td class="muted">{{typeOptions.find((o) => o.value === l.type)?.label ?? l.type}}</td>
                            <td class="ellipsis" :title="l.content">{{ l.content || '—' }}</td>
                        </tr>
                    </tbody>
                </n-table>
            </n-spin>
            <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="reload" />
        </n-card>
    </div>
</template>

<style scoped>
.type-select {
    width: 150px;
}
</style>
