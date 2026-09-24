// 接口契约类型（手工维护，不自动生成）。见开发规范 §4.3。

/** 统一响应体，与后端 pkg/response 对齐。 */
export interface ApiResponse<T = unknown> {
    code: number
    msg: string
    data: T
}

/** 分页结果，与后端 pkg/page 对齐。 */
export interface PageResult<T> {
    items: T[]
    total: number
    pageNum: number
    pageSize: number
    pageTotal: number
}

/** 业务错误码，与后端 pkg/errcode 对齐。 */
export const ErrCode = {
    Success: 200,
    ParamError: 22,
    Failed: 14,
    Unauthorized: 34,
    Forbidden: 403,
} as const

/** 当前登录用户信息。 */
export interface UserInfo {
    id: number
    name: string
    nick_name: string
    email: string
    type: number
    token: string
}
