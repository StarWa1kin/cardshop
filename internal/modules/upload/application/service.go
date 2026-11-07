package application

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/upload/contract"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/google/uuid"
)

var allowedUploadScenes = map[string]struct{}{
	"product":  {},
	"post":     {},
	"banner":   {},
	"editor":   {},
	"common":   {},
	"category": {},
	"telegram": {},
	"reseller": {},
}

// Service 文件上传服务。
type Service struct {
	policy Policy
	store  contract.Store
}

// Policy 定义文件上传校验策略。
type Policy struct {
	MaxSize           int64
	AllowedTypes      []string
	AllowedExtensions []string
	MaxWidth          int
	MaxHeight         int
}

func newUploadValidationError(format string, args ...interface{}) error {
	return &contract.ValidationError{Message: fmt.Sprintf(format, args...)}
}

// NewService 创建文件上传服务实例。
func NewService(policy Policy, store contract.Store) *Service {
	if store == nil {
		panic("upload service: store is nil")
	}
	return &Service{policy: policy, store: store}
}

// inspectedFile 是通过上传校验的文件；调用方负责关闭 src。
type inspectedFile struct {
	src         multipart.File
	ext         string
	contentType string
	width       int
	height      int
}

// inspect 执行上传和替换共用的校验：大小、扩展名、MIME、图片尺寸和 SVG 安全检查。
func (s *Service) inspect(file *multipart.FileHeader, normalizedScene string) (*inspectedFile, error) {
	// 验证文件大小
	if file.Size > s.policy.MaxSize {
		return nil, newUploadValidationError("文件大小超过限制（最大 %d MB）", s.policy.MaxSize/1024/1024)
	}

	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if normalizedScene != "telegram" && len(s.policy.AllowedExtensions) > 0 {
		if ext == "" || !isAllowedExtension(ext, s.policy.AllowedExtensions) {
			return nil, newUploadValidationError("文件扩展名不被允许: %s", ext)
		}
	}

	// 验证文件类型
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	inspected, err := s.inspectContent(src, ext, normalizedScene)
	if err != nil {
		_ = src.Close()
		return nil, err
	}
	return inspected, nil
}

func (s *Service) inspectContent(src multipart.File, ext, normalizedScene string) (*inspectedFile, error) {
	// 读取文件头部识别 MIME 类型
	buffer := make([]byte, 512)
	_, err := src.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if _, err := src.Seek(0, 0); err != nil { // 重置文件读取位置
		return nil, err
	}

	contentType := http.DetectContentType(buffer)
	// http.DetectContentType 无法识别 SVG，需根据扩展名和内容特征补充判断
	if ext == ".svg" && isSVGContent(buffer) {
		contentType = "image/svg+xml"
	}
	if normalizedScene != "telegram" && len(s.policy.AllowedTypes) > 0 {
		allowed := false
		for _, t := range s.policy.AllowedTypes {
			if strings.EqualFold(contentType, t) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, newUploadValidationError("文件类型不被允许: %s", contentType)
		}
	}

	var imgWidth, imgHeight int
	if strings.HasPrefix(contentType, "image/") && contentType != "image/svg+xml" {
		if _, err := src.Seek(0, 0); err != nil {
			return nil, err
		}
		width, height, err := decodeImageDimensions(src, contentType)
		if err != nil {
			return nil, newUploadValidationError("%s", err.Error())
		}
		imgWidth = width
		imgHeight = height
		if s.policy.MaxWidth > 0 && width > s.policy.MaxWidth {
			return nil, newUploadValidationError("图片宽度超过限制（最大 %d）", s.policy.MaxWidth)
		}
		if s.policy.MaxHeight > 0 && height > s.policy.MaxHeight {
			return nil, newUploadValidationError("图片高度超过限制（最大 %d）", s.policy.MaxHeight)
		}
	}

	// SVG 安全检查：禁止嵌入脚本和外部引用
	if contentType == "image/svg+xml" {
		if _, err := src.Seek(0, 0); err != nil {
			return nil, err
		}
		svgData, err := io.ReadAll(src)
		if err != nil {
			return nil, err
		}
		if err := validateSVGSafety(svgData); err != nil {
			return nil, newUploadValidationError("%s", err.Error())
		}
	}

	if _, err := src.Seek(0, 0); err != nil {
		return nil, err
	}
	return &inspectedFile{src: src, ext: ext, contentType: contentType, width: imgWidth, height: imgHeight}, nil
}

// SaveFileWithMeta 保存上传的文件并返回完整元数据
func (s *Service) SaveFileWithMeta(file *multipart.FileHeader, scene string) (*contract.Result, error) {
	normalizedScene := normalizeUploadScene(scene)
	inspected, err := s.inspect(file, normalizedScene)
	if err != nil {
		return nil, err
	}
	defer inspected.src.Close()

	// 生成唯一文件名
	filename := fmt.Sprintf("%s%s", uuid.New().String(), inspected.ext)
	now := time.Now()
	year := now.Format("2006")
	month := now.Format("01")

	storedURL, err := s.store.Save(contract.StoreInput{
		Source:   inspected.src,
		Scene:    normalizedScene,
		Year:     year,
		Month:    month,
		Filename: filename,
	})
	if err != nil {
		return nil, err
	}

	return &contract.Result{
		URL:      storedURL,
		Filename: file.Filename,
		MimeType: inspected.contentType,
		Size:     file.Size,
		Width:    inspected.width,
		Height:   inspected.height,
	}, nil
}

// ReplaceFileWithMeta 用新上传的文件原地覆盖已有文件，公开 URL 保持不变。
// 校验规则与普通上传一致；新文件格式必须与原文件扩展名一致，否则同一地址会返回与后缀不符的内容。
func (s *Service) ReplaceFileWithMeta(file *multipart.FileHeader, publicURL, scene string) (*contract.Result, error) {
	replacer, ok := s.store.(contract.Replacer)
	if !ok {
		return nil, fmt.Errorf("upload service: store does not support replacing files")
	}
	normalizedScene := normalizeUploadScene(scene)
	targetExt := strings.ToLower(path.Ext(publicURL))
	newExt := strings.ToLower(filepath.Ext(file.Filename))
	if canonicalExtension(targetExt) != canonicalExtension(newExt) {
		return nil, newUploadValidationError("新文件格式（%s）与原素材（%s）不一致，为保持图片地址不变，请上传相同格式的文件", displayExtension(newExt), displayExtension(targetExt))
	}

	inspected, err := s.inspect(file, normalizedScene)
	if err != nil {
		return nil, err
	}
	defer inspected.src.Close()

	// 扩展名一致但内容是其它格式（例如改了后缀的 JPG）时同样拒绝。
	if expected, known := imageTypeByExtension[canonicalExtension(targetExt)]; known && inspected.contentType != expected {
		return nil, newUploadValidationError("文件内容（%s）与原素材格式（%s）不一致，请上传相同格式的文件", inspected.contentType, displayExtension(targetExt))
	}

	if err := replacer.Replace(publicURL, inspected.src); err != nil {
		return nil, err
	}
	return &contract.Result{
		URL:      publicURL,
		Filename: file.Filename,
		MimeType: inspected.contentType,
		Size:     file.Size,
		Width:    inspected.width,
		Height:   inspected.height,
	}, nil
}

var imageTypeByExtension = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// canonicalExtension 把同一格式的不同写法归一，例如 .jpeg 与 .jpg。
func canonicalExtension(ext string) string {
	ext = strings.ToLower(ext)
	if ext == ".jpeg" {
		return ".jpg"
	}
	return ext
}

func displayExtension(ext string) string {
	if ext == "" {
		return "无扩展名"
	}
	return ext
}

func normalizeUploadScene(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "common"
	}
	if _, ok := allowedUploadScenes[value]; ok {
		return value
	}
	return "common"
}

func isAllowedExtension(ext string, allowed []string) bool {
	for _, allowedExt := range allowed {
		normalized := strings.ToLower(strings.TrimSpace(allowedExt))
		if normalized == "" {
			continue
		}
		if !strings.HasPrefix(normalized, ".") {
			normalized = "." + normalized
		}
		if strings.EqualFold(ext, normalized) {
			return true
		}
	}
	return false
}

func decodeImageDimensions(src io.ReadSeeker, contentType string) (int, int, error) {
	if strings.EqualFold(contentType, "image/webp") {
		width, height, err := decodeWebPDimensions(src)
		if err != nil {
			return 0, 0, fmt.Errorf("无法解析 WebP 图片: %w", err)
		}
		return width, height, nil
	}

	if _, err := src.Seek(0, 0); err != nil {
		return 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(src)
	if err != nil {
		return 0, 0, fmt.Errorf("无法解析图片: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// isSVGContent 通过文件内容判断是否为 SVG
func isSVGContent(buf []byte) bool {
	content := strings.TrimSpace(string(buf))
	// SVG 文件通常以 XML 声明或 <svg 标签开头
	return strings.HasPrefix(content, "<?xml") ||
		strings.HasPrefix(content, "<svg") ||
		strings.Contains(content, "<svg")
}

// validateSVGSafety 检查 SVG 内容安全性，禁止脚本和危险元素
func validateSVGSafety(data []byte) error {
	content := strings.ToLower(string(data))
	// 禁止脚本标签
	if strings.Contains(content, "<script") {
		return fmt.Errorf("SVG 文件不允许包含 <script> 标签")
	}
	// 禁止事件处理属性（onclick, onload, onerror 等）
	dangerousAttrs := []string{
		"onload", "onclick", "onerror", "onmouseover", "onmouseout",
		"onmousemove", "onfocus", "onblur", "onchange", "onsubmit",
		"onanimationstart", "onanimationend", "onanimationiteration",
	}
	for _, attr := range dangerousAttrs {
		if strings.Contains(content, attr+"=") || strings.Contains(content, attr+" =") {
			return fmt.Errorf("SVG 文件不允许包含事件处理属性: %s", attr)
		}
	}
	// 禁止 javascript: 协议
	if strings.Contains(content, "javascript:") {
		return fmt.Errorf("SVG 文件不允许包含 javascript: 协议")
	}
	// 禁止 data: URI（可用于绕过 CSP）
	if strings.Contains(content, "data:text/html") || strings.Contains(content, "data:application") {
		return fmt.Errorf("SVG 文件不允许包含危险的 data: URI")
	}
	// 禁止 foreignObject（可嵌入 HTML）
	if strings.Contains(content, "<foreignobject") {
		return fmt.Errorf("SVG 文件不允许包含 <foreignObject> 元素")
	}
	return nil
}

func decodeWebPDimensions(src io.ReadSeeker) (int, int, error) {
	if _, err := src.Seek(0, 0); err != nil {
		return 0, 0, err
	}

	header := make([]byte, 12)
	if _, err := io.ReadFull(src, header); err != nil {
		return 0, 0, err
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WEBP" {
		return 0, 0, fmt.Errorf("无效的 WebP 文件头")
	}

	for {
		chunkHeader := make([]byte, 8)
		if _, err := io.ReadFull(src, chunkHeader); err != nil {
			return 0, 0, err
		}
		chunkType := string(chunkHeader[0:4])
		chunkSize := int(binary.LittleEndian.Uint32(chunkHeader[4:8]))
		if chunkSize < 0 {
			return 0, 0, fmt.Errorf("无效的 WebP chunk")
		}

		data := make([]byte, chunkSize)
		if _, err := io.ReadFull(src, data); err != nil {
			return 0, 0, err
		}

		if chunkType == "VP8X" {
			if len(data) < 10 {
				return 0, 0, fmt.Errorf("VP8X chunk 长度不足")
			}
			width := 1 + int(data[4]) + int(data[5])<<8 + int(data[6])<<16
			height := 1 + int(data[7]) + int(data[8])<<8 + int(data[9])<<16
			return width, height, nil
		}
		if chunkType == "VP8 " {
			if len(data) < 10 {
				return 0, 0, fmt.Errorf("VP8 chunk 长度不足")
			}
			width := int(binary.LittleEndian.Uint16(data[6:8]) & 0x3FFF)
			height := int(binary.LittleEndian.Uint16(data[8:10]) & 0x3FFF)
			return width, height, nil
		}
		if chunkType == "VP8L" {
			if len(data) < 5 {
				return 0, 0, fmt.Errorf("VP8L chunk 长度不足")
			}
			if data[0] != 0x2f {
				return 0, 0, fmt.Errorf("VP8L 签名无效")
			}
			bits := binary.LittleEndian.Uint32(data[1:5])
			width := int(bits&0x3FFF) + 1
			height := int((bits>>14)&0x3FFF) + 1
			return width, height, nil
		}

		if chunkSize%2 == 1 {
			if _, err := src.Seek(1, io.SeekCurrent); err != nil {
				return 0, 0, err
			}
		}
	}
}
