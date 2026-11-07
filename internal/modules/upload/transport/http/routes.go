package uploadhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.POST("/upload", handler.UploadFile)
}

// RegisterAdminMediaReplaceRoutes 注册“替换素材文件”路由。
// 调用方必须传入已经挂载认证、RBAC 和后台安全中间件的 RouterGroup。
func RegisterAdminMediaReplaceRoutes(admin gin.IRoutes, handler *MediaReplaceHandler) {
	admin.PUT("/media/:id/file", handler.ReplaceMediaFile)
}
