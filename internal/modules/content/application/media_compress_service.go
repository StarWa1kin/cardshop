package application

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
)

// compressibleTypes 是允许压缩的扩展名及其对应的真实文件类型。
var compressibleTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
}

// MediaCompressResult 是单个素材压缩后的结果。
type MediaCompressResult struct {
	Media        *domain.Media `json:"media"`
	BeforeSize   int64         `json:"before_size"`
	AfterSize    int64         `json:"after_size"`
	SavedBytes   int64         `json:"saved_bytes"`
	SavedPercent float64       `json:"saved_percent"`
	// Replaced 为 false 表示压缩结果不比原图小，原文件保持不变。
	Replaced         bool `json:"replaced"`
	CompressionCount int  `json:"compression_count"`
}

// MediaCompressService 通过外部压缩服务压缩素材图片，并原地替换文件以保持 URL 不变。
type MediaCompressService struct {
	store      contract.MediaStore
	files      contract.MediaFileReplacer
	compressor contract.ImageCompressor
	keys       contract.CompressorKeyProvider
	now        func() time.Time

	inflightMu sync.Mutex
	inflight   map[uint]struct{}
}

// NewMediaCompressService 创建素材压缩服务。
func NewMediaCompressService(
	store contract.MediaStore,
	files contract.MediaFileReplacer,
	compressor contract.ImageCompressor,
	keys contract.CompressorKeyProvider,
) *MediaCompressService {
	if store == nil || files == nil || compressor == nil || keys == nil {
		panic("media compress service: required dependency is nil")
	}
	return &MediaCompressService{
		store:      store,
		files:      files,
		compressor: compressor,
		keys:       keys,
		now:        time.Now,
		inflight:   make(map[uint]struct{}),
	}
}

// Compress 压缩指定素材。只有压缩后更小才会替换原文件。
func (s *MediaCompressService) Compress(ctx context.Context, id uint) (*MediaCompressResult, error) {
	if !s.acquire(id) {
		return nil, contract.ErrCompressInProgress
	}
	defer s.release(id)

	media, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if media == nil {
		return nil, contract.ErrMediaNotFound
	}

	expectedType, ok := compressibleTypes[strings.ToLower(path.Ext(media.Path))]
	if !ok {
		return nil, contract.ErrCompressUnsupportedFormat
	}
	diskPath, err := mediaDiskPath(media.Path)
	if err != nil {
		return nil, err
	}

	apiKey, err := s.keys.GetTinyPNGAPIKey()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, contract.ErrCompressKeyMissing
	}

	original, err := s.files.ReadFile(diskPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, contract.ErrMediaFileMissing
		}
		return nil, err
	}
	// 扩展名可能与真实内容不符（例如改名的 GIF），以文件头为准再校验一次。
	if http.DetectContentType(original) != expectedType {
		return nil, contract.ErrCompressUnsupportedFormat
	}

	compressed, err := s.compressor.Compress(ctx, apiKey, original)
	if err != nil {
		return nil, err
	}
	if compressed == nil || len(compressed.Data) == 0 {
		return nil, fmt.Errorf("%w: empty result", contract.ErrCompressFailed)
	}
	// 文件按原路径替换，输出格式必须与原图一致，否则扩展名和内容会对不上。
	if outputType := http.DetectContentType(compressed.Data); outputType != expectedType {
		return nil, fmt.Errorf("%w: unexpected output type %s", contract.ErrCompressFailed, outputType)
	}

	beforeSize := int64(len(original))
	afterSize := int64(len(compressed.Data))
	replaced := afterSize < beforeSize
	if replaced {
		if err := s.files.ReplaceFile(diskPath, compressed.Data); err != nil {
			return nil, err
		}
	} else {
		afterSize = beforeSize
	}

	now := s.now()
	if media.OriginalSize == 0 {
		media.OriginalSize = beforeSize
	}
	media.Size = afterSize
	media.CompressedAt = &now
	if err := s.store.Update(ctx, media); err != nil {
		return nil, err
	}

	saved := beforeSize - afterSize
	return &MediaCompressResult{
		Media:            media,
		BeforeSize:       beforeSize,
		AfterSize:        afterSize,
		SavedBytes:       saved,
		SavedPercent:     savedPercent(beforeSize, saved),
		Replaced:         replaced,
		CompressionCount: compressed.CompressionCount,
	}, nil
}

func (s *MediaCompressService) acquire(id uint) bool {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	if _, busy := s.inflight[id]; busy {
		return false
	}
	s.inflight[id] = struct{}{}
	return true
}

func (s *MediaCompressService) release(id uint) {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	delete(s.inflight, id)
}

// mediaDiskPath 把 /uploads/... 形式的素材 URL 转成相对工作目录的磁盘路径，并拒绝越出 uploads 目录的路径。
func mediaDiskPath(publicPath string) (string, error) {
	cleaned := path.Clean("/" + strings.TrimSpace(publicPath))
	if !strings.HasPrefix(cleaned, "/uploads/") {
		return "", fmt.Errorf("%w: invalid media path", contract.ErrMediaFileMissing)
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func savedPercent(before, saved int64) float64 {
	if before <= 0 || saved <= 0 {
		return 0
	}
	return math.Round(float64(saved)*10000/float64(before)) / 100
}
