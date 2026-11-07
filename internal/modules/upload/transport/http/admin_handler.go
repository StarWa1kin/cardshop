package uploadhttp

import (
	"context"
	"errors"
	"io/fs"
	"mime/multipart"

	"github.com/dujiao-next/internal/logger"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	uploadcontract "github.com/dujiao-next/internal/modules/upload/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

// FileUploader 是文件落盘端口。
type FileUploader interface {
	SaveFileWithMeta(file *multipart.FileHeader, scene string) (*uploadcontract.Result, error)
}

// MediaRecorder 是上传 HTTP 消费方所需的最小 Content 写入接口。
type MediaRecorder interface {
	RecordMedia(ctx context.Context, result contentapp.UploadResult, scene string) (*contentdomain.Media, error)
}

// AdminHandler 处理后台文件上传请求。
type AdminHandler struct {
	uploader FileUploader
	media    MediaRecorder
}

func NewAdminHandler(uploader FileUploader, media MediaRecorder) *AdminHandler {
	if uploader == nil || media == nil {
		panic("upload admin handler: required dependency is nil")
	}
	return &AdminHandler{uploader: uploader, media: media}
}

// UploadFile 文件上传
func (h *AdminHandler) UploadFile(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.file_missing", nil)
		return
	}
	scene := c.DefaultPostForm("scene", "common")

	result, err := h.uploader.SaveFileWithMeta(file, scene)
	if err != nil {
		if isUploadValidationError(err) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.upload_failed", err)
		return
	}

	var mediaID uint
	media, err := h.media.RecordMedia(c.Request.Context(), contentapp.UploadResult{
		URL:      result.URL,
		Filename: result.Filename,
		MimeType: result.MimeType,
		Size:     result.Size,
		Width:    result.Width,
		Height:   result.Height,
	}, scene)
	if err != nil {
		logger.Warnw("upload_record_media_failed", "error", err, "url", result.URL)
	} else if media != nil {
		mediaID = media.ID
	}

	response.Success(c, gin.H{
		"url":      result.URL,
		"filename": result.Filename,
		"size":     result.Size,
		"media_id": mediaID,
	})
}

func isUploadValidationError(err error) bool {
	var marker interface{ UploadValidationError() }
	return errors.As(err, &marker)
}

// FileReplacer 用新上传的文件原地覆盖已有文件。
type FileReplacer interface {
	ReplaceFileWithMeta(file *multipart.FileHeader, publicURL, scene string) (*uploadcontract.Result, error)
}

// MediaReplacementRecorder 是替换素材文件所需的 Content 能力。
type MediaReplacementRecorder interface {
	Get(ctx context.Context, id uint) (*contentdomain.Media, error)
	RecordReplacement(ctx context.Context, id uint, result contentapp.UploadResult) (*contentdomain.Media, error)
}

// MediaReplaceHandler 处理后台“替换素材文件”请求。
type MediaReplaceHandler struct {
	files FileReplacer
	media MediaReplacementRecorder
}

// NewMediaReplaceHandler 创建素材替换 Handler。
func NewMediaReplaceHandler(files FileReplacer, media MediaReplacementRecorder) *MediaReplaceHandler {
	return &MediaReplaceHandler{files: files, media: media}
}

// ReplaceMediaFile 上传新文件覆盖素材原文件，素材地址保持不变。
func (h *MediaReplaceHandler) ReplaceMediaFile(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_id", nil)
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.file_missing", nil)
		return
	}

	ctx := c.Request.Context()
	media, err := h.media.Get(ctx, id)
	if err != nil {
		if errors.Is(err, contentcontract.ErrMediaNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.media_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}

	result, err := h.files.ReplaceFileWithMeta(file, media.Path, media.Scene)
	if err != nil {
		switch {
		case isUploadValidationError(err):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		case errors.Is(err, fs.ErrNotExist):
			ginutil.RespondError(c, response.CodeNotFound, "error.media_file_missing", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.upload_failed", err)
		}
		return
	}

	updated, err := h.media.RecordReplacement(ctx, id, contentapp.UploadResult{
		URL:      result.URL,
		Filename: result.Filename,
		MimeType: result.MimeType,
		Size:     result.Size,
		Width:    result.Width,
		Height:   result.Height,
	})
	if err != nil {
		// 文件已经替换成功，只是元数据没更新；返回错误让管理员重试，重试会再次覆盖同一文件。
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, updated)
}
