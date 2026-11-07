package contenthttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/dujiao-next/internal/i18n"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

// MediaCompressor 是素材压缩接口所需的用例。
type MediaCompressor interface {
	Compress(ctx context.Context, id uint) (*contentapp.MediaCompressResult, error)
}

// MediaCompressHandler 处理后台素材压缩请求。
type MediaCompressHandler struct {
	compressor MediaCompressor
}

// NewMediaCompressHandler 创建素材压缩 Handler。
func NewMediaCompressHandler(compressor MediaCompressor) *MediaCompressHandler {
	return &MediaCompressHandler{compressor: compressor}
}

// compressErrorMapping 把压缩错误映射为业务码、文案 key 和前端可识别的 error_code。
var compressErrorMapping = []struct {
	err       error
	code      int
	key       string
	errorCode string
}{
	{contentcontract.ErrMediaNotFound, response.CodeNotFound, "error.media_not_found", "media_not_found"},
	{contentcontract.ErrMediaFileMissing, response.CodeNotFound, "error.media_file_missing", "media_file_missing"},
	{contentcontract.ErrCompressUnsupportedFormat, response.CodeBadRequest, "error.media_compress_unsupported", "unsupported_format"},
	{contentcontract.ErrCompressKeyMissing, response.CodeBadRequest, "error.media_compress_key_missing", "tinypng_key_missing"},
	// TinyPNG 的 401 不能映射成业务 401，否则前端会当作登录失效踢回登录页。
	{contentcontract.ErrCompressUnauthorized, response.CodeBadRequest, "error.media_compress_unauthorized", "tinypng_unauthorized"},
	{contentcontract.ErrCompressQuotaExceeded, response.CodeTooManyRequests, "error.media_compress_quota_exceeded", "tinypng_quota_exceeded"},
	{contentcontract.ErrCompressTimeout, response.CodeInternal, "error.media_compress_timeout", "tinypng_timeout"},
	{contentcontract.ErrCompressInProgress, response.CodeBadRequest, "error.media_compress_in_progress", "compress_in_progress"},
	{contentcontract.ErrCompressFailed, response.CodeInternal, "error.media_compress_failed", "compress_failed"},
}

// CompressMedia 调用 TinyPNG 压缩素材图片，并原地替换文件。
func (h *MediaCompressHandler) CompressMedia(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_id", nil)
		return
	}
	result, err := h.compressor.Compress(c.Request.Context(), id)
	if err != nil {
		respondCompressError(c, err)
		return
	}
	response.Success(c, result)
}

func respondCompressError(c *gin.Context, err error) {
	code, key, errorCode := response.CodeInternal, "error.media_compress_failed", "compress_failed"
	for _, mapping := range compressErrorMapping {
		if errors.Is(err, mapping.err) {
			code, key, errorCode = mapping.code, mapping.key, mapping.errorCode
			break
		}
	}
	message := i18n.T(i18n.ResolveLocale(c), key)
	if code >= response.CodeInternal {
		ginutil.RequestLog(c).Errorw("media_compress_failed", "error", err)
	} else {
		ginutil.RequestLog(c).Warnw("media_compress_rejected", "error", err)
	}
	c.JSON(http.StatusOK, response.Response{
		StatusCode: code,
		Msg:        message,
		Data:       gin.H{"error_code": errorCode},
	})
}
