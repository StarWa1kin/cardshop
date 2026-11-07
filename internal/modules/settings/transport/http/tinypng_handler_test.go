package settingshttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
)

type tinyPNGSettingsStub struct {
	setting settingsintegration.TinyPNGSetting
}

func (s *tinyPNGSettingsStub) GetTinyPNGSetting() (settingsintegration.TinyPNGSetting, error) {
	return s.setting, nil
}

func (s *tinyPNGSettingsStub) PatchTinyPNGSetting(patch settingsintegration.TinyPNGSettingPatch) (settingsintegration.TinyPNGSetting, error) {
	s.setting = settingsintegration.ApplyTinyPNGSettingPatch(s.setting, patch)
	return s.setting, nil
}

func performTinyPNGRequest(t *testing.T, handler gin.HandlerFunc, method, body string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, "/api/v1/admin/settings/tinypng", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	handler(context)
	var got struct {
		StatusCode int                    `json:"status_code"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if got.StatusCode != 0 {
		t.Fatalf("unexpected status: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-key-1234567890") {
		t.Fatalf("plaintext key leaked: %s", recorder.Body.String())
	}
	return got.Data
}

func TestTinyPNGHandlerMasksKeyAndKeepsItOnEmptyPatch(t *testing.T) {
	stub := &tinyPNGSettingsStub{}
	handler := NewTinyPNGHandler(stub)

	data := performTinyPNGRequest(t, handler.UpdateTinyPNG, http.MethodPut, `{"api_key":"  secret-key-1234567890 "}`)
	if data["has_api_key"] != true || data["api_key"] != "" || data["api_key_masked"] != "****7890" {
		t.Fatalf("masked response mismatch: %#v", data)
	}
	if stub.setting.APIKey != "secret-key-1234567890" {
		t.Fatalf("key not trimmed/saved: %q", stub.setting.APIKey)
	}

	performTinyPNGRequest(t, handler.UpdateTinyPNG, http.MethodPut, `{"api_key":""}`)
	if stub.setting.APIKey != "secret-key-1234567890" {
		t.Fatal("empty api_key should keep the stored key")
	}

	data = performTinyPNGRequest(t, handler.GetTinyPNG, http.MethodGet, "")
	if data["has_api_key"] != true {
		t.Fatalf("get response mismatch: %#v", data)
	}

	data = performTinyPNGRequest(t, handler.UpdateTinyPNG, http.MethodPut, `{"clear_api_key":true}`)
	if data["has_api_key"] != false || stub.setting.APIKey != "" {
		t.Fatalf("clear failed: %#v", data)
	}
}

func TestAdminHandlerRejectsTinyPNGOnGenericEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &adminSettingsStub{}
	handler := NewAdminHandler(stub)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings?key=tinypng_config", nil)
	handler.Get(context)
	if !strings.Contains(recorder.Body.String(), `"status_code":400`) {
		t.Fatalf("generic get should reject tinypng_config: %s", recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", strings.NewReader(`{"key":"tinypng_config","value":{"api_key":"x"}}`))
	context.Request.Header.Set("Content-Type", "application/json")
	handler.Update(context)
	if !strings.Contains(recorder.Body.String(), `"status_code":400`) || stub.updateCalls != 0 {
		t.Fatalf("generic update should reject tinypng_config: %s", recorder.Body.String())
	}
}
