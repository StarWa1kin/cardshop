package settingshttp

import (
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// TinyPNGAdminService 是后台 TinyPNG 设置端口。
type TinyPNGAdminService interface {
	GetTinyPNGSetting() (settingsintegration.TinyPNGSetting, error)
	PatchTinyPNGSetting(patch settingsintegration.TinyPNGSettingPatch) (settingsintegration.TinyPNGSetting, error)
}

// TinyPNGHandler 处理后台 TinyPNG 设置请求，响应中不返回明文 Key。
type TinyPNGHandler struct {
	settings TinyPNGAdminService
}

func NewTinyPNGHandler(settings TinyPNGAdminService) *TinyPNGHandler {
	if settings == nil {
		panic("settings tinypng handler: settings is nil")
	}
	return &TinyPNGHandler{settings: settings}
}

// GetTinyPNG 获取 TinyPNG 配置（脱敏）。
func (h *TinyPNGHandler) GetTinyPNG(c *gin.Context) {
	setting, err := h.settings.GetTinyPNGSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingsintegration.MaskTinyPNGSettingForAdmin(setting))
}

// UpdateTinyPNG 更新 TinyPNG 配置。
func (h *TinyPNGHandler) UpdateTinyPNG(c *gin.Context) {
	var req settingsintegration.TinyPNGSettingPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	setting, err := h.settings.PatchTinyPNGSetting(req)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}
	response.Success(c, settingsintegration.MaskTinyPNGSettingForAdmin(setting))
}
