// 语言切换：本地立即生效（i18n + localStorage），已登录时同时写回用户表（跨设备一致）。
import { updateProfile } from '@/api/auth'
import { applyUserLocale, setLocale, type Locale } from '@/i18n'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'

export function useLocaleSwitch(): (locale: Locale) => Promise<void> {
  const user = useUserStore()
  const message = useMessage()
  const { t } = useI18n()

  return async function switchLocale(locale: Locale): Promise<void> {
    if (locale === user.userInfo?.lang) return
    setLocale(locale)
    // 提示用新语言：先切 locale 再取文案。
    message.success(t('user.languageChanged'))
    if (!user.isLogged) return
    try {
      // 不带 id = 改自己（后端缺省回退当前登录用户）。
      const res = await updateProfile(undefined, { lang: locale })
      if (res.code === ErrCode.Success && res.data) user.setUser({ ...res.data, token: user.token })
      else message.warning(res.msg || t('user.languageFailed'))
    } catch {
      message.warning(t('user.languageFailed'))
    }
  }
}

/** 登录后/刷新时按用户表偏好应用语言（空串表示未设置，保持本地选择）。 */
export function syncLocaleFromUser(lang?: string | null): void {
  applyUserLocale(lang)
}
