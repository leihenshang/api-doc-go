// i18n 基础设施：语言来源优先级 = 用户表 lang（登录后同步）> localStorage > 浏览器语言 > 中文兜底。
// 组件内用 `const { t } = useI18n()`（全局作用域）取文案；模板里可直接 `{{ t('x.y') }}`。
import { createI18n } from 'vue-i18n'
import enUS from './locales/en-US'
import zhCN from './locales/zh-CN'

/** 支持的语言（与后端 model.LangZhCN / LangEnUS 白名单一致，改这里需同步后端）。 */
export const LOCALES = ['zh-CN', 'en-US'] as const
export type Locale = (typeof LOCALES)[number]

/** 兜底语言：语言包缺 key 时回落，也用于首次访问判定不出语言的情况。 */
export const FALLBACK_LOCALE: Locale = 'zh-CN'

/** 语言本地存储键（未登录时也要记住选择）。 */
const STORAGE_KEY = 'apidoc.lang'

/** 语言显示名（切换入口用，本身不翻译）。 */
export const LOCALE_LABELS: Record<Locale, string> = {
  'zh-CN': '简体中文',
  'en-US': 'English',
}

/** 是否受支持的语言值。 */
export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && (LOCALES as readonly string[]).includes(value)
}

/** 初始语言：localStorage → 浏览器语言（zh* 视为中文，其余英文）→ 兜底。 */
export function detectLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (isLocale(saved)) return saved
  const nav = typeof navigator === 'undefined' ? '' : navigator.language
  return nav?.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: detectLocale(),
  fallbackLocale: FALLBACK_LOCALE,
  messages: { 'zh-CN': zhCN, 'en-US': enUS },
})

/** 当前语言（响应式读取，可用于 computed）。 */
export function currentLocale(): Locale {
  const value = i18n.global.locale.value
  return isLocale(value) ? value : FALLBACK_LOCALE
}

/**
 * 切换界面语言：改 i18n、写 localStorage、同步 <html lang>。
 * 持久化到用户表由调用方负责（需登录，见 composables/locale.ts）。
 */
export function setLocale(locale: Locale): void {
  i18n.global.locale.value = locale
  localStorage.setItem(STORAGE_KEY, locale)
  if (typeof document !== 'undefined') document.documentElement.setAttribute('lang', locale)
}

/** 应用用户表里的语言偏好；空串/非法值表示未设置，保持现状。 */
export function applyUserLocale(lang?: string | null): void {
  if (isLocale(lang)) setLocale(lang)
}

/**
 * 非组件环境取文案（utils / 无 setup 的 composable）——走全局实例。
 * 组件内仍优先 `useI18n()`，以便语言切换时自动重渲染。
 */
export function translate(key: string, named?: Record<string, unknown>): string {
  const fn = i18n.global.t as unknown as (k: string, n?: Record<string, unknown>) => string
  return fn(key, named ?? {})
}

// 首屏同步 <html lang>（无障碍与浏览器翻译提示依赖它）。
if (typeof document !== 'undefined') document.documentElement.setAttribute('lang', currentLocale())
