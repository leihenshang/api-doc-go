// 用户状态：登录信息 + token，持久化 localStorage。按模块拆分 store。
import type { User } from '@/types/entities'
import { defineStore } from 'pinia'

/** 登录用户：User 附加 token。 */
export type LoginUser = User & { token: string }

const STORAGE_KEY = 'userInfo'

function loadFromStorage(): LoginUser | null {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as LoginUser) : null
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
