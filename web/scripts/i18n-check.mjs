// i18n 语言包自查（供 `npm run i18n:check` 使用）：
// ① zh-CN 与 en-US 的 key 是否一一对应 ② 源码里 t('x.y') / translate('x.y') 用到的 key 是否都在语言包里。
// 用法：node scripts/i18n-check.mjs [web 目录，缺省为当前工作目录]
import { readFileSync, readdirSync } from 'node:fs'
import { join, relative, sep } from 'node:path'

const webDir = process.argv[2] ?? process.cwd()
const localeDir = join(webDir, 'src/i18n/locales')

/** 加载纯数据 TS 语言包（无类型标注，去掉 export default 后按 CommonJS 求值）。 */
function loadLocale(name) {
  const src = readFileSync(join(localeDir, `${name}.ts`), 'utf8').replace('export default', 'module.exports =')
  const mod = { exports: {} }
  new Function('module', 'exports', src)(mod, mod.exports)
  return mod.exports
}

/** 展平成 a.b.c 形式的 key 列表。 */
function flatten(obj, prefix = '', out = []) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object') flatten(v, key, out)
    else out.push(key)
  }
  return out
}

const zh = flatten(loadLocale('zh-CN'))
const en = flatten(loadLocale('en-US'))
const zhSet = new Set(zh)
const enSet = new Set(en)
const onlyZh = zh.filter((k) => !enSet.has(k))
const onlyEn = en.filter((k) => !zhSet.has(k))

/** 递归收集源码文件（排除语言包自身）。 */
function walk(dir, out = []) {
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, e.name)
    if (e.isDirectory()) walk(full, out)
    else if (/\.(vue|ts)$/.test(e.name) && !full.includes(`${sep}i18n${sep}`)) out.push(full)
  }
  return out
}

const used = new Map()
// 同时匹配 t('k')、$t('k')、translate('k')：非组件代码（utils/composable）只能用 translate，
// 此前只扫 t( 会漏掉这类调用，缺 key 也照常退出 0。
const USAGE = /\b(?:t|translate)\(\s*['"]([\w.-]+)['"]/g
for (const file of walk(join(webDir, 'src'))) {
  for (const m of readFileSync(file, 'utf8').matchAll(USAGE)) {
    if (!used.has(m[1])) used.set(m[1], [])
    used.get(m[1]).push(relative(webDir, file))
  }
}

const missing = [...used.keys()].filter((k) => !zhSet.has(k) || !enSet.has(k))
console.log(`zh-CN keys: ${zh.length} | en-US keys: ${en.length}`)
console.log(`仅中文有: ${onlyZh.length ? onlyZh.join(', ') : '无'}`)
console.log(`仅英文有: ${onlyEn.length ? onlyEn.join(', ') : '无'}`)
console.log(`源码用到但语言包缺失: ${missing.length ? missing.map((k) => `${k} (${used.get(k)[0]})`).join(', ') : '无'}`)
process.exit(onlyZh.length || onlyEn.length || missing.length ? 1 : 0)
