package settingsstorefront

import (
	"errors"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrInvalidTheme = errors.New("theme_id must be theme-1, theme-2 or theme-3")

func IsThemeID(raw interface{}) bool {
	value, ok := raw.(string)
	return ok && (value == "theme-1" || value == "theme-2" || value == "theme-3")
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
