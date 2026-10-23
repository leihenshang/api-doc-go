// Package migrations 以 embed.FS 打包 SQL 迁移脚本（审核 H-14）。
//
// 脚本仍放在 migrations/{sqlite,postgres}/，由 internal/database 的迁移执行器读取；
// 打包进二进制后，部署只需一个可执行文件，不会再出现「脚本在仓库里、生产从未执行」。
package migrations

import "embed"

// FS 包含两个方言目录下的全部 .sql 迁移脚本。
//
//go:embed sqlite/*.sql postgres/*.sql
var FS embed.FS
