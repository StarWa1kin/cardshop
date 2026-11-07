package settingsintegration

import (
	"strings"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// TinyPNGSetting 是素材压缩使用的 TinyPNG 配置。
type TinyPNGSetting struct {
	APIKey string `json:"api_key"`
}

// TinyPNGSettingPatch 是后台提交的 TinyPNG 配置补丁。
// APIKey 为空字符串时保持原值，ClearAPIKey 为 true 时清空已保存的 Key。
type TinyPNGSettingPatch struct {
	APIKey      *string `json:"api_key"`
	ClearAPIKey bool    `json:"clear_api_key"`
}

// NormalizeTinyPNGSetting 去掉 Key 两端空白。
func NormalizeTinyPNGSetting(setting TinyPNGSetting) TinyPNGSetting {
	setting.APIKey = strings.TrimSpace(setting.APIKey)
	return setting
}

// DecodeTinyPNGSetting 从持久化 JSON 解码 TinyPNG 配置。
func DecodeTinyPNGSetting(raw jsonmap.JSON) TinyPNGSetting {
	setting := TinyPNGSetting{}
	if raw == nil {
		return setting
	}
	if text, ok := raw["api_key"].(string); ok {
		setting.APIKey = text
	}
	return NormalizeTinyPNGSetting(setting)
}

// EncodeTinyPNGSetting 把 TinyPNG 配置编码为持久化 JSON。
func EncodeTinyPNGSetting(setting TinyPNGSetting) jsonmap.JSON {
	normalized := NormalizeTinyPNGSetting(setting)
	return jsonmap.JSON{"api_key": normalized.APIKey}
}

// ApplyTinyPNGSettingPatch 把补丁应用到当前配置。
func ApplyTinyPNGSettingPatch(current TinyPNGSetting, patch TinyPNGSettingPatch) TinyPNGSetting {
	next := NormalizeTinyPNGSetting(current)
	if patch.ClearAPIKey {
		next.APIKey = ""
	}
	if patch.APIKey != nil {
		if key := strings.TrimSpace(*patch.APIKey); key != "" {
			next.APIKey = key
		}
	}
	return next
}

// MaskTinyPNGSettingForAdmin 返回脱敏后的配置，只透出末 4 位用于辨认。
func MaskTinyPNGSettingForAdmin(setting TinyPNGSetting) jsonmap.JSON {
	normalized := NormalizeTinyPNGSetting(setting)
	return jsonmap.JSON{
		"api_key":        "",
		"has_api_key":    normalized.APIKey != "",
		"api_key_masked": maskSecretTail(normalized.APIKey),
	}
}

func maskSecretTail(secret string) string {
	if secret == "" {
		return ""
	}
	runes := []rune(secret)
	if len(runes) <= 8 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}
