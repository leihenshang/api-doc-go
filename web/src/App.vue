<script setup lang="ts">
// 根组件：Naive UI 全局配置（中文语料 + 主题变量）+ 消息/对话框 provider + 路由出口。
import type { GlobalThemeOverrides } from 'naive-ui';
import { NConfigProvider, NDialogProvider, NMessageProvider, dateZhCN, zhCN } from 'naive-ui';

/** 主题微调：主色沿用绿色系（与旧版高亮一致），收紧圆角与卡片内边距。 */
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#18a058',
    primaryColorHover: '#36ad6a',
    primaryColorPressed: '#0c7a43',
    primaryColorSuppl: '#18a058',
    borderRadius: '4px',
    borderRadiusSmall: '3px',
  },
  Card: {
    paddingMedium: '16px 20px',
    borderRadiusMedium: '4px',
  },
  DataTable: {
    thColor: '#fafbfc',
    thFontWeight: '500',
  },
}
</script>

<template>
  <n-config-provider :locale="zhCN" :date-locale="dateZhCN" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
