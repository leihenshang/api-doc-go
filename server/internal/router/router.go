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

	// 公共接口（无需登录）。
	engine.GET("/api/health", h.Health)
	engine.POST("/api/user/login", h.Login)
	engine.POST("/api/user/register", h.Register)
	// 查重供注册页未登录时调用。
	engine.GET("/api/user/check-repeat", h.CheckRepeat)

	// 静态资源（上传目录）。
	engine.Static("/uploads", cfg.Upload.Dir)

	// 受保护接口。
	auth := engine.Group("/api", middleware.Auth(cfg.JWT.Secret))
	registerRoutes(auth, h)
	return engine
}

// registerRoutes 注册需鉴权的路由。
func registerRoutes(g *gin.RouterGroup, h *handler.Handler) {
	// 用户
	{
		u := g.Group("/user")
		u.GET("/me", h.Me)
		u.GET("/list", h.ListUsers)
		u.PUT("/profile", h.Update)
		u.PUT("/password", h.UpdatePwd)
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
	// 接口文档
	{
		a := g.Group("/api")
		a.POST("", h.CreateApi)
		a.GET("", h.ListApis)
		a.GET("/detail", h.GetApi)
		a.PUT("", h.UpdateApi)
		a.DELETE("", h.DeleteApi)
		a.POST("/restore", h.RestoreApi)
		a.POST("/import", h.ImportOpenAPI)
		a.GET("/export", h.ExportOpenAPI)
		a.GET("/curl", h.ExportCurl)
	}
	// 项目文档
	{
		d := g.Group("/doc")
		d.POST("", h.CreateDoc)
		d.GET("", h.ListDocs)
		d.GET("/detail", h.GetDoc)
		d.PUT("", h.UpdateDoc)
		d.DELETE("", h.DeleteDoc)
		d.POST("/restore", h.RestoreDoc)
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
	{
		d := g.Group("/database")
		d.POST("", h.CreateConn)
		d.GET("", h.ListConns)
		d.PUT("", h.UpdateConn)
		d.DELETE("", h.DeleteConn)
		d.GET("/tables", h.ConnTables)
		d.GET("/columns", h.ConnColumns)
	}
	// 文件上传
	g.POST("/file/upload", h.UploadImg)
}
