package analyticshttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	analyticsapp "github.com/dujiao-next/internal/modules/analytics/application"
	analyticsdomain "github.com/dujiao-next/internal/modules/analytics/domain"

	"github.com/gin-gonic/gin"
)

type collectorStub struct {
	err    error
	inputs []analyticsapp.TrackInput
}

func (s *collectorStub) Track(input analyticsapp.TrackInput) error {
	s.inputs = append(s.inputs, input)
	return s.err
}

func performTrackRequest(t *testing.T, collector Collector, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterPublicRoutes(engine, NewPublicHandler(collector))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/visits", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "test-agent")
	engine.ServeHTTP(recorder, request)
	return recorder
}

// 本项目统一用 HTTP 200 承载业务状态码，所以断言要看响应体里的 status_code。
func businessStatusCode(t *testing.T, recorder *httptest.ResponseRecorder) int {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected transport status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v (%s)", err, recorder.Body.String())
	}
	return payload.StatusCode
}

func TestTrackVisitForwardsRequestContext(t *testing.T) {
	collector := &collectorStub{}
	recorder := performTrackRequest(t, collector, `{"scope":"product","target":"my-slug","visitor_key":"abc"}`)

	if code := businessStatusCode(t, recorder); code != 0 {
		t.Fatalf("expected success, got status_code %d: %s", code, recorder.Body.String())
	}
	if len(collector.inputs) != 1 {
		t.Fatalf("expected one tracked visit, got %d", len(collector.inputs))
	}
	input := collector.inputs[0]
	if input.Scope != analyticsdomain.ScopeProduct || input.ProductSlug != "my-slug" || input.VisitorKey != "abc" {
		t.Fatalf("unexpected input: %+v", input)
	}
	if input.UserAgent != "test-agent" || input.ClientIP == "" {
		t.Fatalf("client context must be forwarded, got %+v", input)
	}
}

func TestTrackVisitRejectsInvalidScopeAndTarget(t *testing.T) {
	for _, testCase := range []struct {
		name string
		err  error
	}{
		{name: "invalid scope", err: analyticsdomain.ErrScopeInvalid},
		{name: "invalid target", err: analyticsdomain.ErrTargetInvalid},
		{name: "missing target", err: analyticsdomain.ErrTargetNotFound},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := performTrackRequest(t, &collectorStub{err: testCase.err}, `{"scope":"home"}`)
			if code := businessStatusCode(t, recorder); code != http.StatusBadRequest {
				t.Fatalf("expected status_code 400, got %d: %s", code, recorder.Body.String())
			}
		})
	}
}

// 埋点不是业务流程：落库异常只记日志，前台不应该看到失败。
func TestTrackVisitHidesPersistenceFailuresFromStorefront(t *testing.T) {
	recorder := performTrackRequest(t, &collectorStub{err: errors.New("database unavailable")}, `{"scope":"home"}`)
	if code := businessStatusCode(t, recorder); code != 0 {
		t.Fatalf("storefront must not see persistence failures, got status_code %d", code)
	}
}

func TestTrackVisitRejectsMalformedPayload(t *testing.T) {
	recorder := performTrackRequest(t, &collectorStub{}, `{"target":"x"}`)
	if code := businessStatusCode(t, recorder); code == 0 {
		t.Fatal("missing scope must be rejected")
	}
}
