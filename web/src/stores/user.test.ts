// 用户 store：本地登录态解析必须抗损坏数据（W-16）。
// 这是整个应用启动路径上的第一段代码，抛错会直接白屏，故单独锁死行为。
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useUserStore } from '@/stores/user'

const KEY = 'userInfo'

describe('user store 本地登录态', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('损坏的 JSON 不抛异常，清理脏数据并按未登录处理', () => {
    localStorage.setItem(KEY, '{"token": "abc"')
    expect(() => useUserStore()).not.toThrow()
    const store = useUserStore()
    expect(store.userInfo).toBeNull()
    expect(store.isLogged).toBe(false)
    // 脏数据必须被清掉，否则每次刷新都要再走一遍这个异常
    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('合法 JSON 但缺 token 视为未登录', () => {
    localStorage.setItem(KEY, JSON.stringify({ id: 1, name: 'bob' }))
    const store = useUserStore()
    expect(store.userInfo).toBeNull()
    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('token 为空串同样视为未登录', () => {
    localStorage.setItem(KEY, JSON.stringify({ id: 1, name: 'bob', token: '' }))
    expect(useUserStore().isLogged).toBe(false)
  })

  it('合法登录态正常读入，管理员判定按 type 2/3', () => {
    localStorage.setItem(KEY, JSON.stringify({ id: 1, name: 'bob', nick_name: '鲍勃', token: 't', type: 2 }))
    const store = useUserStore()
    expect(store.isLogged).toBe(true)
    expect(store.token).toBe('t')
    expect(store.nickName).toBe('鲍勃')
    expect(store.isAdmin).toBe(true)
  })

  it('clear 后本地存储一并清空', () => {
    localStorage.setItem(KEY, JSON.stringify({ id: 1, token: 't', type: 1 }))
    const store = useUserStore()
    store.clear()
    expect(store.userInfo).toBeNull()
    expect(localStorage.getItem(KEY)).toBeNull()
  })
})
