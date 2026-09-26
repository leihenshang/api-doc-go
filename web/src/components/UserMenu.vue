<script setup lang="ts">
// 顶栏用户区：头像悬停出二级菜单（修改资料 / 修改密码 / 语言 / 退出登录），
// 点选后由全局弹窗承接（改密与退出确认都居中弹层）。语言项为二级菜单，直接切换界面语言。
import PasswordModal from '@/components/PasswordModal.vue'
import ProfileModal from '@/components/ProfileModal.vue'
import { useLocaleSwitch } from '@/composables/locale'
import { LOCALE_LABELS, LOCALES, currentLocale, isLocale, type Locale } from '@/i18n'
import { useUserStore } from '@/stores/user'
import { CheckmarkOutline, CreateOutline, KeyOutline, LanguageOutline, LogOutOutline } from '@vicons/ionicons5'
import { NAvatar, NDropdown, NIcon, useDialog } from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import { computed, h, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const emit = defineEmits<{ (e: 'logout'): void }>()

const user = useUserStore()
const dialog = useDialog()
const { t } = useI18n()
const switchLocale = useLocaleSwitch()
const initials = computed(() => user.nickName.slice(0, 1).toUpperCase() || 'U')

const showProfile = ref(false)
const showPwd = ref(false)

/** 语言二级菜单：当前语言带对勾。key 形如 `lang:en-US`。 */
const langOptions = computed<DropdownOption[]>(() => {
  const active = currentLocale()
  return LOCALES.map((code) => ({
    label: LOCALE_LABELS[code],
    key: `lang:${code}`,
    icon: code === active ? () => h(NIcon, { component: CheckmarkOutline }) : undefined,
  }))
})

const options = computed<DropdownOption[]>(() => [
  { label: t('user.profile'), key: 'profile', icon: () => h(NIcon, { component: CreateOutline }) },
  { label: t('user.password'), key: 'pwd', icon: () => h(NIcon, { component: KeyOutline }) },
  {
    label: t('user.language'),
    key: 'lang',
    icon: () => h(NIcon, { component: LanguageOutline }),
    children: langOptions.value,
  },
  { label: t('user.logout'), key: 'logout', icon: () => h(NIcon, { component: LogOutOutline }) },
])

function onSelect(key: string | number): void {
  const value = String(key)
  if (value === 'profile') showProfile.value = true
  else if (value === 'pwd') showPwd.value = true
  else if (value.startsWith('lang:')) {
    const code = value.slice(5)
    if (isLocale(code)) void switchLocale(code as Locale)
  } else if (value === 'logout') confirmLogout()
}

/** 退出登录同样用居中弹窗二次确认。 */
function confirmLogout(): void {
  dialog.warning({
    title: t('user.logout'),
    content: t('user.logoutConfirm'),
    positiveText: t('user.logout'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => emit('logout'),
  })
}
</script>

<template>
  <span class="user-menu">
    <n-dropdown trigger="hover" placement="bottom-end" :options="options" @select="onSelect">
      <span class="account">
        <n-avatar round :size="28" class="avatar">{{ initials }}</n-avatar>
        <span class="who">{{ t('user.hello', { name: user.nickName }) }}</span>
      </span>
    </n-dropdown>
  </span>
  <profile-modal v-model:show="showProfile" />
  <password-modal v-model:show="showPwd" />
</template>

<style scoped>
.account {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.avatar {
  background: var(--app-accent);
  color: #fff;
}

.who {
  color: var(--app-text);
  font-size: 13px;
}
</style>
