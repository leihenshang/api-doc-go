// Package handler HTTP 层：绑定请求、取当前主体、调用 service、写出统一响应。
package handler

import (
	"errors"
	"log"
	"strconv"

	"apidoc/internal/config"
	"apidoc/internal/middleware"
	"apidoc/internal/pkg/errcode"
	"apidoc/internal/pkg/response"
	"apidoc/internal/repository"
	"apidoc/internal/service"

	"github.com/gin-gonic/gin"
)

// Handler 聚合依赖。
type Handler struct {
	cfg *config.Config
	svc *service.Service
}

// New 构造 Handler。
func New(cfg *config.Config, svc *service.Service) *Handler {
	return &Handler{cfg: cfg, svc: svc}
}

// principal 取当前登录主体。
func principal(c *gin.Context) middleware.Principal {
	p, _ := middleware.Current(c)
	return p
}

// fail 将 service 层错误映射为统一响应。
func fail(c *gin.Context, err error) {
	var be *service.BizError
	if errors.As(err, &be) {
		// 业务错误对外的信息是 Msg，底层原因仅落日志，避免泄漏 SQL 细节
		if be.Cause != nil {
			log.Printf("handler: %v", be.Cause)
		}
		response.Fail(c, be.Code, be.Msg)
		return
	}
	if errors.Is(err, repository.ErrNotFound) {
		response.Fail(c, errcode.CodeFailed, "数据不存在")
		return
	}
	// 非业务错误原文可能含 SQL/驱动细节：只落日志，对外统一文案
	log.Printf("handler: %v", err)
	response.Fail(c, errcode.CodeFailed, "操作失败")
}

// bindJSON 绑定请求体；失败写参数错误并返回 false。
func bindJSON(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		response.Fail(c, errcode.CodeParamError, "参数错误: "+err.Error())
		return false
	}
	return true
}

// bindQuery 绑定查询参数；失败写参数错误并返回 false。
func bindQuery(c *gin.Context, req any) bool {
	if err := c.ShouldBindQuery(req); err != nil {
		response.Fail(c, errcode.CodeParamError, "参数错误: "+err.Error())
		return false
	}
	return true
}

// idParam 解析资源 id（取自查询参数 id）；无效返回 0。
// 统一采用静态路径 + 查询参数，避免同方法下静态段与 :param 段的树冲突。
func idParam(c *gin.Context) uint64 {
	v, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	return v
}

// idQuery 解析查询参数 name；无效返回 0。
func idQuery(c *gin.Context, name string) uint64 {
	v, _ := strconv.ParseUint(c.Query(name), 10, 64)
	return v
}

// idOrFallback 解析业务主键：优先查询参数 name（如 project_id、bug_id），
// 缺失时回落到通用 id，兼容不同客户端写法。契约以 doc/api_docs 为准。
func idOrFallback(c *gin.Context, name string) uint64 {
	if id := idQuery(c, name); id > 0 {
		return id
	}
	return idParam(c)
}
