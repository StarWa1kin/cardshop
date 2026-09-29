package settingsapp

import (
	"errors"
	"reflect"
	"testing"

	"github.com/dujiao-next/internal/constants"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestDefaultSettingRegistryCoversLegacyNormalizedKeys(t *testing.T) {
	want := []string{
		constants.SettingKeyAffiliateConfig,
		constants.SettingKeyCallbackRoutesConfig,
		constants.SettingKeyDashboardConfig,
		constants.SettingKeyGoogleAuthConfig,
		constants.SettingKeyHomeAnnouncement,
		constants.SettingKeyNavConfig,
		constants.SettingKeyNotificationCenterConfig,
		constants.SettingKeyOrderConfig,
		constants.SettingKeyOrderRiskControlConfig,
		constants.SettingKeyPaymentConfig,
		constants.SettingKeyRegistrationConfig,
		constants.SettingKeySiteConfig,
		constants.SettingKeyStorefrontThemeConfig,
		constants.SettingKeyTelegramAuthConfig,
		constants.SettingKeyTelegramBotConfig,
		constants.SettingKeyUpstreamSyncConfig,
		constants.SettingKeyWalletConfig,
	}

	if got := defaultSettingRegistry.Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("default registry keys mismatch\nwant: %#v\n got: %#v", want, got)
	}
}

func TestSettingServiceUpdateWithEffectsDescribesCacheImpact(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      map[string]interface{}
		wantEffect Effect
	}{
		{
			name:       "google auth config invalidates public cache",
			key:        constants.SettingKeyGoogleAuthConfig,
			value:      map[string]interface{}{"enabled": true, "client_id": "client.apps.googleusercontent.com"},
			wantEffect: EffectInvalidatePublicConfigCache,
		},
		{
			name:       "site config invalidates public cache",
			key:        constants.SettingKeySiteConfig,
			value:      map[string]interface{}{},
			wantEffect: EffectInvalidatePublicConfigCache,
		},
		{
			name:       "wallet config is effect only",
			key:        constants.SettingKeyWalletConfig,
			value:      map[string]interface{}{constants.SettingFieldWalletOnlyPayment: true},
			wantEffect: EffectInvalidatePublicConfigCache,
		},
		{
			name:       "callback routes invalidate route cache",
			key:        constants.SettingKeyCallbackRoutesConfig,
			value:      map[string]interface{}{constants.SettingFieldPaymentCallback: "/api/custom/callback"},
			wantEffect: EffectInvalidateCallbackRoutesCache,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newMockSettingRepo()
			service := NewService(repo)
			result, err := service.UpdateWithEffects(test.key, test.value)
			if err != nil {
				t.Fatalf("update setting: %v", err)
			}
			if !result.HasEffect(test.wantEffect) {
				t.Fatalf("expected effect %q, got %#v", test.wantEffect, result.Effects)
			}
		})
	}

	repo := newMockSettingRepo()
	service := NewService(repo)
	result, err := service.UpdateWithEffects("custom_extension_config", map[string]interface{}{"enabled": true})
	if err != nil {
		t.Fatalf("update unknown setting: %v", err)
	}
	if len(result.Effects) != 0 {
		t.Fatalf("unknown setting emitted effects: %#v", result.Effects)
	}
}

func TestSettingServiceUpdateKeepsUnknownKeyPassThroughBehavior(t *testing.T) {
	repo := newMockSettingRepo()
	service := NewService(repo)
	input := map[string]interface{}{
		"nested": map[string]interface{}{"enabled": true},
		"count":  float64(3),
	}

	got, err := service.Update("custom_extension_config", input)
	if err != nil {
		t.Fatalf("update unknown setting: %v", err)
	}
	if !reflect.DeepEqual(got, jsonmap.JSON(input)) {
		t.Fatalf("unknown setting changed during pass-through: %#v", got)
	}
	if saved := repo.store["custom_extension_config"]; !reflect.DeepEqual(saved, jsonmap.JSON(input)) {
		t.Fatalf("unknown setting persisted with a different shape: %#v", saved)
	}
}

func TestPaymentFeeConfigDefaultsSafeAndNormalizesSwitches(t *testing.T) {
	repo := newMockSettingRepo()
	service := NewService(repo)

	if got := service.GetPaymentFeeConfig(); got != (PaymentFeeConfig{}) {
		t.Fatalf("default payment fee config must be safe: %#v", got)
	}

	stored, err := service.Update(constants.SettingKeyPaymentConfig, map[string]interface{}{
		constants.SettingFieldCustomerFeeEnabled:         "true",
		constants.SettingFieldReuseLegacyOrderFeePayment: 1,
		"unexpected": true,
	})
	if err != nil {
		t.Fatalf("update payment fee config: %v", err)
	}
	want := jsonmap.JSON{
		constants.SettingFieldCustomerFeeEnabled:         true,
		constants.SettingFieldReuseLegacyOrderFeePayment: true,
	}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("normalized payment config mismatch: %#v", stored)
	}
	if got := service.GetPaymentFeeConfig(); got != (PaymentFeeConfig{CustomerFeeEnabled: true, ReuseLegacyOrderFeePayment: true}) {
		t.Fatalf("payment fee config mismatch: %#v", got)
	}
}

func TestStorefrontThemeRoundtripAndRollback(t *testing.T) {
	repo := newMockSettingRepo()
	service := NewService(repo)
	original := jsonmap.JSON{"storefront_template": "vault", "template_mode": "list", "currency": "USD", "unchanged": "keep"}
	repo.store[constants.SettingKeySiteConfig] = original
	read := func(want string) {
		t.Helper()
		got, err := service.GetConfig(map[string]interface{}{})
		if err != nil {
			t.Fatal(err)
		}
		if got[constants.SettingFieldStorefrontTheme] != want {
			t.Fatalf("config=%v", got)
		}
		for k, v := range original {
			if !reflect.DeepEqual(got[k], v) {
				t.Fatalf("legacy field changed: %s", k)
			}
		}
	}
	read("theme-2")
	saved, err := service.UpdateWithEffects(constants.SettingKeyStorefrontThemeConfig, map[string]interface{}{"theme_id": "theme-3", "extra": "discard"})
	if err != nil {
		t.Fatal(err)
	}
	if !saved.HasEffect(EffectInvalidatePublicConfigCache) {
		t.Fatal("missing cache invalidation")
	}
	if len(saved.Value) != 1 {
		t.Fatal("unexpected saved keys")
	}
	read("theme-3")
	if _, err := service.Update(constants.SettingKeyStorefrontThemeConfig, map[string]interface{}{"theme_id": "bad"}); !errors.Is(err, settingsstorefront.ErrInvalidTheme) {
		t.Fatalf("validation error: %v", err)
	}
	read("theme-3")
	// Independent site writes never erase the new setting.
	if _, err := service.Update(constants.SettingKeySiteConfig, map[string]interface{}(original)); err != nil {
		t.Fatal(err)
	}
	read("theme-3")
	// Normal configuration rollback; no business state is touched.
	if _, err := service.Update(constants.SettingKeyStorefrontThemeConfig, map[string]interface{}{"theme_id": "theme-2"}); err != nil {
		t.Fatal(err)
	}
	read("theme-2")
	delete(repo.store, constants.SettingKeyStorefrontThemeConfig)
	read("theme-2")
}

func TestThemeLoadsWithoutSiteConfig(t *testing.T) {
	repo := newMockSettingRepo()
	service := NewService(repo)
	if _, err := service.Update(constants.SettingKeyStorefrontThemeConfig, map[string]interface{}{"theme_id": "theme-3"}); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got[constants.SettingFieldStorefrontTheme] != "theme-3" {
		t.Fatalf("early return lost theme: %v", got)
	}
	repo.store[constants.SettingKeyStorefrontThemeConfig] = jsonmap.JSON{"theme_id": "corrupt"}
	got, err = service.GetConfig(map[string]interface{}{"storefront_template": "vault"})
	if err != nil {
		t.Fatal(err)
	}
	if got[constants.SettingFieldStorefrontTheme] != "theme-2" {
		t.Fatalf("invalid stored theme: %v", got)
	}
}
