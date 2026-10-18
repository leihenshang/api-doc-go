// 统一 HTTP 客户端：注入凭证、集中处理业务码。页面不裸调 axios（见开发规范 §4.3）。
import { useUserStore } from '@/stores/user'
import type { ApiResponse } from '@/types/api'
import { ErrCode } from '@/types/api'
import axios, { type AxiosInstance } from 'axios'

/**
 * 未登录（业务码 34）时的跳转动作，由 main.ts 注入。
 *
 * 这里**不能**静态 `import router`：router 会动态 import 各页面，页面又 import 本文件，
 * 形成「router → 视图 → http → router」的环。生产构建下首次导航会对这个环里的 chunk
 * 发起 import()，而该 chunk 的依赖又在等 router 求值完成 → 模块图死锁，
 * 表现就是「带登录态的刷新/深链接进受保护页永远白屏，且没有任何报错」（P0-1）。
 * 改成注入后，http 不再依赖 router，环断开。
 */
type UnauthorizedHandler = () => void
let unauthorizedHandler: UnauthorizedHandler = () => {}

/** 注册未登录跳转（main.ts 在装配完 router 后调用一次）。 */
export function setUnauthorizedHandler(fn: UnauthorizedHandler): void {
    unauthorizedHandler = fn
}

const http: AxiosInstance = axios.create({
    // 统一走版本化前缀（P2-27）：新增破坏性接口时用 /api/v2，旧版继续跑 /api/v1。
    baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
    timeout: 15000,
})

// 请求拦截：自动携带 token（沿用旧约定，终态可切 Authorization 头）。
http.interceptors.request.use((config) => {
    const user = useUserStore()
    if (user.token) {
        config.headers.set('Authorization', `Bearer ${user.token}`)
    }
    return config
})

// 响应拦截：集中处理 34（未登录/过期）跳登录；其余原样返回，由 get/post 解包。
http.interceptors.response.use(
    (response) => {
        const body = response.data as ApiResponse
        if (body && body.code === ErrCode.Unauthorized) {
            useUserStore().clear()
            unauthorizedHandler()
        }
        return response
    },
    (error) => Promise.reject(error),
)

/** 发起 GET 并返回已解包的统一响应体。 */
export async function get<T>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
    const res = await http.get<ApiResponse<T>>(url, { params })
    return res.data
}

/** 发起 POST 并返回已解包的统一响应体。 */
export async function post<T>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    const res = await http.post<ApiResponse<T>>(url, data)
    return res.data
}

/** 发起 PUT 并返回已解包的统一响应体。 */
export async function put<T>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    const res = await http.put<ApiResponse<T>>(url, data)
    return res.data
}

/** 发起 DELETE（可带查询串）并返回已解包的统一响应体。 */
export async function del<T>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
    const res = await http.delete<ApiResponse<T>>(url, { params })
    return res.data
}

export default http
