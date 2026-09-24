<script setup lang="ts">
// 列表分页条：统一「共 N 条 + 页码」呈现。数据不足一页时也保留，
// 避免各列表分页位置有无不一致。
import { NPagination } from 'naive-ui';

withDefaults(defineProps<{ total: number; pageNum: number; pageSize?: number }>(), { pageSize: 20 })

const emit = defineEmits<{ 'update:pageNum': [value: number] }>()
</script>

<template>
  <div class="pager">
    <span class="meta">共 {{ total }} 条</span>
    <n-pagination
      :page="pageNum"
      :page-size="pageSize"
      :item-count="total"
      @update:page="emit('update:pageNum', $event)"
    />
  </div>
</template>

<style scoped>
.pager {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  /* 卡片为纵向 flex 时贴底，位置不随表格行数变化 */
  margin-top: auto;
  padding: 12px 16px;
  border-top: 1px solid var(--app-border);
}
</style>
