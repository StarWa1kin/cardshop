package settingsstorefront

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestThemeCompatibility(t *testing.T) {
	for _, test := range []struct {
		theme, legacy interface{}
		want          string
	}{
		{nil, nil, "theme-1"}, {nil, "classic", "theme-1"}, {nil, "vault", "theme-2"},
		{"theme-1", "vault", "theme-1"}, {"theme-2", "classic", "theme-2"}, {"theme-3", "vault", "theme-3"}, {"theme-4", "classic", "theme-4"},
		{"bad", "vault", "theme-2"}, {123, "unknown", "theme-1"}, {[]string{}, "vault", "theme-2"},
	} {
		if got := ResolveTheme(test.theme, test.legacy); got != test.want {
			t.Fatalf("resolve(%v,%v)=%s want %s", test.theme, test.legacy, got, test.want)
		}
	}
}
func TestThemeValidation(t *testing.T) {
	for _, value := range []interface{}{nil, "", "theme-5", "Theme-4", " theme-3 ", 123, true, []string{"theme-3"}} {
		if !errors.Is(ValidateThemeSetting(jsonmap.JSON{"theme_id": value}), ErrInvalidTheme) {
			t.Fatalf("accepted %v", value)
		}
	}
	for _, value := range []string{"theme-1", "theme-2", "theme-3", "theme-4"} {
		if err := ValidateThemeSetting(jsonmap.JSON{"theme_id": value}); err != nil {
			t.Fatal(err)
		}
	}
}
