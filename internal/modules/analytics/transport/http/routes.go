package analyticshttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册前台访问上报路由。
func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	if public == nil || handler == nil {
		panic("analytics public routes: required dependency is nil")
	}
	public.POST("/visits", handler.TrackVisit)
}

// RegisterAdminRoutes 注册后台访问统计路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("analytics admin routes: required dependency is nil")
	}
	admin.GET("/analytics/product-views", handler.GetProductViews)
}
