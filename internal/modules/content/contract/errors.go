package contract

import "errors"

var (
	ErrNotFound                      = errors.New("not found")
	ErrSlugExists                    = errors.New("slug exists")
	ErrCategoryParentInvalid         = errors.New("category parent invalid")
	ErrCategoryInUse                 = errors.New("category in use")
	ErrInvalidPostType               = errors.New("invalid post type")
	ErrPostCategoryInvalid           = errors.New("post category invalid")
	ErrPostNoticeCategoryUnsupported = errors.New("post notice category unsupported")
	ErrInvalidBanner                 = errors.New("invalid banner")
	ErrMediaNotFound                 = errors.New("media not found")
	ErrMediaNameEmpty                = errors.New("media name empty")
	ErrMediaFileMissing              = errors.New("media file missing")

	// 素材压缩相关错误。
	ErrCompressUnsupportedFormat = errors.New("media compress: unsupported format")
	ErrCompressKeyMissing        = errors.New("media compress: tinypng api key not configured")
	ErrCompressUnauthorized      = errors.New("media compress: tinypng api key rejected")
	ErrCompressQuotaExceeded     = errors.New("media compress: tinypng quota exceeded")
	ErrCompressTimeout           = errors.New("media compress: tinypng request timed out")
	ErrCompressInProgress        = errors.New("media compress: already in progress")
	ErrCompressFailed            = errors.New("media compress: failed")
)
