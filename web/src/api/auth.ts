// 用户/鉴权接口。
import type { ApiResponse } from '@/types/api'
import type { CreateTokenResp, LoginResult, Page, User, UserToken } from '@/types/entities'
import { del, get, post, put } from './http'

export interface LoginPayload {
    name: string
    pwd: string
}

export function login(payload: LoginPayload): Promise<ApiResponse<LoginResult>> {
    return post<LoginResult>('/user/login', payload)
}

export function register(payload: {
    name: string
    pwd: string
    re_pwd: string
    email?: string
    nick_name?: string
}): Promise<ApiResponse<User>> {
    return post<User>('/user/register', payload)
}

/** 注册前查重：keyword 是否已被用户名/邮箱/昵称占用。 */
export function checkRepeat(keyword: string): Promise<ApiResponse<{ repeated: boolean }>> {
    return get<{ repeated: boolean }>('/user/check-repeat', { keyword })
}

export function me(): Promise<ApiResponse<User>> {
    return get<User>('/user/me')
}

export function listUsers(params: { keyword?: string; page_num?: number; page_size?: number }): Promise<ApiResponse<Page<User>>> {
    return get<Page<User>>('/user/list', params)
}

/** 更新资料；id 省略（undefined/0）表示改自己（后端按当前登录用户回退）。 */
export function updateProfile(id: number | undefined, payload: Record<string, unknown>): Promise<ApiResponse<User>> {
    return put<User>(`/user/profile${id ? `?id=${id}` : ''}`, payload)
}

export function updatePwd(payload: { old_pwd: string; pwd: string; re_pwd: string }): Promise<ApiResponse<null>> {
    return put<null>('/user/password', payload)
}

// ---- 个人访问令牌（PAT，S7）：供桌面客户端同步鉴权；明文仅创建时返回一次。 ----

/** 创建 PAT。days=0 表示不过期；scopes 目前仅存不校验（建议填 sync）。 */
export function createToken(payload: { name: string; scopes?: string; days?: number }): Promise<ApiResponse<CreateTokenResp>> {
    return post<CreateTokenResp>('/user/token', payload)
}

/** 我的 PAT 列表。 */
export function listTokens(): Promise<ApiResponse<UserToken[]>> {
    return get<UserToken[]>('/user/token')
}

/** 查看本人 PAT 明文（服务端解密；旧令牌密文为空会报错）。 */
export function revealToken(id: number): Promise<ApiResponse<{ token: string }>> {
    return get<{ token: string }>('/user/token/reveal', { id })
}

/** 删除指定 PAT（物理删除，不可恢复；客户端将无法再用它同步）。 */
export function deleteToken(id: number): Promise<ApiResponse<null>> {
    return del<null>('/user/token', { id })
}
