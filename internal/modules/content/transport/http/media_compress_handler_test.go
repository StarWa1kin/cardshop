package contenthttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type compressorStub struct {
	result *contentapp.MediaCompressResult
	err    error
	gotID  uint
}

func (s *compressorStub) Compress(_ context.Context, id uint) (*contentapp.MediaCompressResult, error) {
	s.gotID = id
	return s.result, s.err
}

type compressResponse struct {
	StatusCode int                    `json:"status_code"`
	Msg        string                 `json:"msg"`
	Data       map[string]interface{} `json:"data"`
}

func performCompress(t *testing.T, compressor MediaCompressor, id string) compressResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterAdminMediaCompressRoutes(router.Group("/api/v1/admin"), NewMediaCompressHandler(compressor))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/admin/media/"+id+"/compress", nil))
	var got compressResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	return got
}

func TestCompressMediaMapsErrorsToBusinessCodes(t *testing.T) {
	tests := []struct {
		err       error
		code      int
		errorCode string
	}{
		{fmt.Errorf("wrapped: %w", contentcontract.ErrCompressQuotaExceeded), response.CodeTooManyRequests, "tinypng_quota_exceeded"},
		{contentcontract.ErrCompressKeyMissing, response.CodeBadRequest, "tinypng_key_missing"},
		{contentcontract.ErrCompressUnauthorized, response.CodeBadRequest, "tinypng_unauthorized"},
		{contentcontract.ErrCompressUnsupportedFormat, response.CodeBadRequest, "unsupported_format"},
		{contentcontract.ErrCompressTimeout, response.CodeInternal, "tinypng_timeout"},
		{contentcontract.ErrMediaNotFound, response.CodeNotFound, "media_not_found"},
		{fmt.Errorf("disk full"), response.CodeInternal, "compress_failed"},
	}
	for _, test := range tests {
		t.Run(test.errorCode, func(t *testing.T) {
			got := performCompress(t, &compressorStub{err: test.err}, "5")
			if got.StatusCode != test.code || got.Data["error_code"] != test.errorCode || got.Msg == "" {
				t.Fatalf("want %d/%s got %#v", test.code, test.errorCode, got)
			}
		})
	}
}

func TestCompressMediaRejectsInvalidID(t *testing.T) {
	stub := &compressorStub{}
	got := performCompress(t, stub, "abc")
	if got.StatusCode != response.CodeBadRequest || stub.gotID != 0 {
		t.Fatalf("invalid id should be rejected: %#v", got)
	}
}
