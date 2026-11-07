package lotteryhttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册用户侧抽奖路由。调用方必须传入已通过用户鉴权的路由组，
// 游客没有 user_id，因此结构上无法参与抽奖。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("lottery user routes: required dependency is nil")
	}
	user.GET("/lottery/wheel", handler.GetWheel)
	// 路径不放在 /lottery/orders/ 下：静态段会和同层的 :order_no 通配冲突，gin 会直接 panic。
	user.GET("/lottery/drawable-orders", handler.ListDrawableOrders)
	user.GET("/lottery/orders/:order_no/chance", handler.GetChance)
	user.POST("/lottery/orders/:order_no/draw", handler.Draw)
	user.GET("/lottery/draws", handler.ListDraws)
}

// RegisterAdminRoutes 注册后台抽奖配置与中奖记录路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("lottery admin routes: required dependency is nil")
	}
	admin.GET("/lottery/config", handler.GetConfig)
	admin.PUT("/lottery/config", handler.UpdateConfig)
	admin.GET("/lottery/prizes", handler.ListPrizes)
	admin.POST("/lottery/prizes", handler.CreatePrize)
	admin.PUT("/lottery/prizes/:id", handler.UpdatePrize)
	admin.DELETE("/lottery/prizes/:id", handler.DeletePrize)
	admin.GET("/lottery/draws", handler.ListDraws)
	admin.PATCH("/lottery/draws/:id/deliver", handler.MarkDrawDelivered)
}
