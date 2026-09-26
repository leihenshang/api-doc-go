<script setup lang="ts">
// 列表分页条：统一「共 N 条 + 每页条数 + 页码」呈现。数据不足一页时也保留，
// 避免各列表分页位置有无不一致。
// 每页条数：仅当传入 pageSizes 时才显示选择器（默认不显示，既有页面外观不变）。
// 后端 page_size 上限为 200（internal/pkg/page.Normalize）。
import { NPagination } from 'naive-ui';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

withDefaults(
  defineProps<{ total: number; pageNum: number; pageSize?: number; pageSizes?: number[] }>(),
  { pageSize: 20, pageSizes: () => [] },
)

const emit = defineEmits<{
  'update:pageNum': [value: number];
  'update:pageSize': [value: number];
}>()
</script>

<template>
  <div class="pager">
    <span class="meta">{{ t('common.total', { n: total }) }}</span>
    <n-pagination
      :page="pageNum"
      :page-size="pageSize"
      :item-count="total"
      :page-sizes="pageSizes"
      :show-size-picker="pageSizes.length > 0"
      @update:page="emit('update:pageNum', $event)"
      @update:page-size="emit('update:pageSize', $event)"
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

/* 每页条数选择器宽度（naive-ui 默认较窄，长文案会截断） */
.pager :deep(.n-pagination .n-select) {
  width: 112px;
}
</style>
