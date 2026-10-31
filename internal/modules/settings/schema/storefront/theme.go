package settingsstorefront

import (
	"errors"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrInvalidTheme = errors.New("theme_id must be theme-1, theme-2, theme-3 or theme-4")

// themeIDs mirrors STOREFRONT_THEMES in frontend/shared/storefrontTheme.ts.
var themeIDs = map[string]bool{"theme-1": true, "theme-2": true, "theme-3": true, "theme-4": true}

func IsThemeID(raw interface{}) bool {
	value, ok := raw.(string)
	return ok && themeIDs[value]
}

func ValidateThemeSetting(value jsonmap.JSON) error {
	if !IsThemeID(value["theme_id"]) {
		return ErrInvalidTheme
	}
	return nil
}

func NormalizeThemeSetting(value jsonmap.JSON) jsonmap.JSON {
	return jsonmap.JSON{"theme_id": value["theme_id"]}
}

// ResolveTheme preserves the legacy storefront until an explicit theme is saved.
func ResolveTheme(theme interface{}, legacy interface{}) string {
	if IsThemeID(theme) {
		return theme.(string)
	}
	if legacy == "vault" {
		return "theme-2"
	}
	return "theme-1"
}
