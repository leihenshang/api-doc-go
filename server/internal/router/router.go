// Package router 组装 gin 引擎并按业务模块注册路由。
package router

import (
	"apidoc/internal/config"
	"apidoc/internal/handler"
	"apidoc/internal/middleware"
	"apidoc/internal/repository"
	"apidoc/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Setup 创建引擎并注册全部路由。
func Setup(db *gorm.DB, cfg *config.Config) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), middleware.CORS())

	h := handler.New(cfg, service.New(repository.New(db, cfg), cfg))

	// 健康检查注册在限流之前：探活/监控不消耗配额。
	engine.GET("/api/health", h.Health)

	// 全局限流（每 IP 每分钟，0 = 关闭）：防止误用与暴力刷接口。
	engine.Use(middleware.RateLimit(cfg.Security.RateLimitPerMin))

	// 上传目录静态服务：白名单扩展名 + 固定 Content-Type + nosniff（替代 engine.Static）。
	engine.GET("/uploads/*path", h.ServeUpload)
	// Mock 服务（前端联调用，按项目 id 直接对外开放）。
	engine.Any("/mock/:project_id/*path", h.Mock)

	// 两套前缀同时挂载（P2-27）：
	//   /api/v1/...  新代码与 Web 前端用，带版本号，未来破坏性升级有地方安放；
	//   /api/...     历史路径，桌面端已发布版本仍在用，不能拆。
	// v1 里「接口实体」的路径段是 /item（旧的是 /api/api，读起来像重复了一层）。
	for _, m := range []struct{ prefix, entitySeg string }{
		{"/api", "api"},     // 历史兼容
		{"/api/v1", "item"}, // 版本化
	} {
		// 公共接口（无需登录）；登录/注册额外套一层更严格的限流（防爆破）。
		authLimit := middleware.RateLimit(cfg.Security.AuthRateLimitPerMin)
		engine.POST(m.prefix+"/user/login", authLimit, h.Login)
		engine.POST(m.prefix+"/user/register", authLimit, h.Register)
		// 查重供注册页未登录时调用。
		engine.GET(m.prefix+"/user/check-repeat", h.CheckRepeat)
		// 免登录：只读分享数据（令牌校验 + 有效期）。
		engine.GET(m.prefix+"/share/:token", h.SharePayload)
		// 能力协商（客户端连接时校验协议兼容性）。
		engine.GET(m.prefix+"/meta", h.Meta)

		// 受保护接口（JWT 或 PAT）。JWT 路径回源刷新主体：禁用/降权后存量令牌立即失效（审核 H-1）。
		auth := engine.Group(m.prefix, middleware.AuthOrPATWithRefresh(cfg.JWT.Secret, h.VerifyPAT, h.LoadPrincipal))
		registerRoutes(auth, h, m.entitySeg)
	}
	return engine
}

// registerRoutes 注册需鉴权的路由。
func registerRoutes(g *gin.RouterGroup, h *handler.Handler, entitySeg string) {
	// 用户
	{
		u := g.Group("/user")
		u.GET("/me", h.Me)
		u.GET("/list", h.ListUsers)
		u.PUT("/profile", h.Update)
		u.PUT("/password", h.UpdatePwd)
		// PAT（S7）
		u.POST("/token", h.CreateToken)
		u.GET("/token", h.ListTokens)
		u.DELETE("/token", h.RevokeToken)
		u.GET("/token/reveal", h.RevealToken)
	}
	// 同步（S4）
	{
		g.GET("/sync/state", h.SyncState)
		g.GET("/sync/changes", h.SyncChanges)
		g.GET("/sync/snapshot", h.SyncSnapshot)
		g.POST("/sync/push", h.SyncPush)
	}
	// 项目与成员
	{
		g.POST("/project", h.CreateProject)
		g.GET("/project", h.ListProjects)
		g.GET("/project/detail", h.GetProject)
		g.PUT("/project", h.UpdateProject)
		g.DELETE("/project", h.DeleteProject)
		g.GET("/project/members", h.ListMembers)
		g.POST("/project/member", h.AddMember)
		g.PUT("/project/leader", h.SetLeader)
		g.PUT("/project/permission", h.SetPermission)
		g.GET("/project/permission", h.MyPermission)
		g.POST("/project/quit", h.QuitProject)
	}
	// 分组
	{
		gr := g.Group("/group")
		gr.POST("", h.CreateGroup)
		gr.GET("", h.ListGroups)
		gr.PUT("", h.UpdateGroup)
		gr.DELETE("", h.DeleteGroup)
	}
	// 拖拽排序（分组 / 接口 / 文档通用）
	{
		g.POST("/sort", h.SortNodes)
	}
	// 接口文档（路径段随挂载前缀变化：历史 /api/api，v1 /api/v1/item）
	{
		a := g.Group("/" + entitySeg)
		a.POST("", h.CreateApi)
		a.GET("", h.ListApis)
		a.GET("/detail", h.GetApi)
		a.PUT("", h.UpdateApi)
		a.DELETE("", h.DeleteApi)
		// 回收站：恢复进出软删状态，purge 为物理删除
		a.POST("/restore", h.RestoreApi)
		a.POST("/restore-batch", h.RestoreApis)
		a.POST("/purge-batch", h.PurgeApis)
		a.POST("/import", h.ImportOpenAPI)
		a.POST("/import-bruno", h.ImportBruno)
		a.GET("/export", h.ExportOpenAPI)
		a.GET("/curl", h.ExportCurl)
		a.GET("/code", h.CodeSnippet)
		a.GET("/export-postman", h.ExportPostman)
		a.POST("/import-postman", h.ImportPostman)
		a.POST("/sample", h.SaveApiSample)
		a.GET("/samples", h.ListApiSamples)
		a.DELETE("/sample", h.DeleteApiSample)
	}
	// 项目文档
	{
		d := g.Group("/doc")
		d.POST("", h.CreateDoc)
		d.GET("", h.ListDocs)
		d.GET("/groups", h.ListDocGroups)
		d.GET("/detail", h.GetDoc)
		d.PUT("", h.UpdateDoc)
		d.DELETE("", h.DeleteDoc)
		// 回收站：恢复进出软删状态，purge 为物理删除
		d.POST("/restore", h.RestoreDoc)
		d.POST("/restore-batch", h.RestoreDocs)
		d.POST("/purge-batch", h.PurgeDocs)
		d.POST("/like", h.LikeDoc)
	}
	// 字段映射
	{
		f := g.Group("/field-mapping")
		f.POST("", h.CreateFieldMapping)
		f.GET("", h.ListFieldMappings)
		f.PUT("", h.UpdateFieldMapping)
		f.DELETE("", h.DeleteFieldMapping)
	}
	// 环境（项目级多环境：项目变量的取值维度）
	{
		e := g.Group("/variable-env")
		e.POST("", h.CreateVariableEnv)
		e.GET("", h.ListVariableEnvs)
		e.PUT("", h.UpdateVariableEnv)
		e.PUT("/default", h.SetDefaultVariableEnv)
		e.GET("/references", h.VariableEnvRefs)
		e.DELETE("", h.DeleteVariableEnv)
	}
	// 变量（请求级 / 项目级 / 全局）+ 解析表 + 列表徽标批量统计
	{
		v := g.Group("/variable")
		v.POST("", h.CreateVariable)
		v.GET("", h.ListVariables)
		v.PUT("", h.UpdateVariable)
		v.DELETE("", h.DeleteVariable)
		v.GET("/resolve", h.ResolveVariables)
		v.GET("/summary", h.SummarizeVariables)
	}
	// Bug
	{
		b := g.Group("/bug")
		b.POST("", h.CreateBug)
		b.GET("", h.ListBugs)
		b.GET("/detail", h.GetBug)
		b.PUT("", h.UpdateBug)
		b.DELETE("", h.DeleteBug)
		b.POST("/assign", h.AssignBug)
		b.GET("/assign", h.ListBugAssign)
	}
	// 全局搜索（跨项目：接口 / 文档 / 项目）
	g.GET("/search", h.Search)
	// 分享链接（创建/列表/失效；免登录读取见下方公共路由）
	{
		s := g.Group("/share")
		s.POST("", h.CreateShareLink)
		s.GET("", h.ListShareLinks)
		s.DELETE("", h.DeleteShareLink)
	}
	// 公共属性
	{
		p := g.Group("/property")
		p.GET("", h.ListProperties)
		p.POST("", h.SaveProperty)
		p.DELETE("", h.DeleteProperty)
	}
	// 消息 / 通知
	g.POST("/message/send", h.SendMessage)
	g.GET("/notification", h.ListNotifications)
	g.POST("/notification/read", h.MarkNotificationRead)
	// 操作日志
	g.GET("/operation-log", h.ListLogs)
	// 外部数据库连接
	// 文件上传
	g.POST("/file/upload", h.UploadImg)
	// S13 内容寻址：按 sha256 取文件（文档图片离线映射 R9）
	g.GET("/file/:hash", h.GetFileByHash)
}
