// 用户状态：登录信息 + token，持久化 localStorage。按模块拆分 store。
import type { User } from '@/types/entities'
import { defineStore } from 'pinia'

/** 登录用户：User 附加 token。 */
export type LoginUser = User & { token: string }

const STORAGE_KEY = 'userInfo'

/**
 * 读取本地登录态。
 *
 * 这里必须吞掉解析异常：store 的 state 是在模块求值期构造的，
 * 一旦 JSON.parse 抛出，整个 pinia 初始化失败，应用直接白屏且用户
 * 没有任何自助恢复手段（清缓存属于不可发现的路径）。损坏数据只可能
 * 来自手工篡改或旧版本残留，处理方式是丢弃它并当作未登录，让用户
 * 走一次正常登录重新写入。
 */
function loadFromStorage(): LoginUser | null {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    try {
        const parsed = JSON.parse(raw) as Partial<LoginUser> | null
        // 结构校验：token 非空才算有效登录态，否则等同于未登录。
        if (!parsed || typeof parsed.token !== 'string' || !parsed.token) {
            localStorage.removeItem(STORAGE_KEY)
            return null
        }
        return parsed as LoginUser
    } catch {
        localStorage.removeItem(STORAGE_KEY)
        return null
    }
}

export const useUserStore = defineStore('user', {
    state: () => ({
        userInfo: loadFromStorage(),
    }),
    getters: {
        token(state): string {
            return state.userInfo?.token ?? ''
        },
        isLogged(state): boolean {
            return Boolean(state.userInfo?.token)
        },
        nickName(state): string {
            return state.userInfo?.nick_name || state.userInfo?.name || ''
        },
        /** 管理员或超级管理员（type 2/3）。 */
        isAdmin(state): boolean {
            const t = state.userInfo?.type ?? 0
            return t === 2 || t === 3
        },
    },
    actions: {
        /** 保存登录用户并写入本地存储。 */
        setUser(info: LoginUser) {
            this.userInfo = info
            localStorage.setItem(STORAGE_KEY, JSON.stringify(info))
        },
        /** 清空登录态（登出 / 过期时调用）。 */
        clear() {
            this.userInfo = null
            localStorage.removeItem(STORAGE_KEY)
        },
    },
})
