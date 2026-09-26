<script setup lang="ts">
// 根组件：Naive UI 全局配置（语料随 i18n 语言切换 + 主题变量）+ 消息/对话框 provider + 路由出口。
import type { GlobalThemeOverrides } from 'naive-ui';
import { NConfigProvider, NDialogProvider, NMessageProvider, dateEnUS, dateZhCN, enUS, zhCN } from 'naive-ui';
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

// 全局作用域：naive-ui 的内置文案（分页、日期、空态、输入框清除提示等）与界面语言保持一致。
const { locale } = useI18n({ useScope: 'global' });
const isEnglish = computed(() => locale.value === 'en-US');
const naiveLocale = computed(() => (isEnglish.value ? enUS : zhCN));
const naiveDateLocale = computed(() => (isEnglish.value ? dateEnUS : dateZhCN));

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
  <n-config-provider :locale="naiveLocale" :date-locale="naiveDateLocale" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
