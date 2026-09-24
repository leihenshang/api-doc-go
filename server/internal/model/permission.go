package model

// 操作权限位掩码（与 user_project.permission 对应）。
const (
	PermQuery  int8 = 1 // 查询
	PermCreate int8 = 2 // 创建
	PermUpdate int8 = 4 // 更新
	PermDelete int8 = 8 // 删除
)

// PermAll 是读写全权限。
const PermAll = PermQuery | PermCreate | PermUpdate | PermDelete

// PermReadonly 是只读权限。
const PermReadonly = PermQuery

// Has 判断权限掩码是否包含所需位。
func Has(perm, need int8) bool { return perm&need == need }
