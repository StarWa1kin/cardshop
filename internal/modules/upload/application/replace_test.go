package application

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/modules/upload/contract"
)

type replacingStore struct {
	memoryStore
	replacedURL string
	replaced    []byte
}

func (s *replacingStore) Replace(publicURL string, source io.Reader) error {
	data, err := io.ReadAll(source)
	if err != nil {
		return err
	}
	s.replacedURL = publicURL
	s.replaced = data
	return nil
}

func encodeImage(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	var err error
	if format == "png" {
		err = png.Encode(&buffer, img)
	} else {
		err = jpeg.Encode(&buffer, img, nil)
	}
	if err != nil {
		t.Fatalf("encode %s: %v", format, err)
	}
	return buffer.Bytes()
}

func multipartFile(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	form, err := multipart.NewReader(&body, writer.Boundary()).ReadForm(10 << 20)
	if err != nil {
		t.Fatal(err)
	}
	return form.File["file"][0]
}

func replacePolicy() Policy {
	return Policy{
		MaxSize:           1 << 20,
		AllowedTypes:      []string{"image/jpeg", "image/png", "image/webp"},
		AllowedExtensions: []string{".jpg", ".jpeg", ".png", ".webp"},
		MaxWidth:          4096,
		MaxHeight:         4096,
	}
}

func TestReplaceFileWithMetaOverwritesSameURL(t *testing.T) {
	store := &replacingStore{}
	service := NewService(replacePolicy(), store)
	content := encodeImage(t, "png", 30, 20)

	result, err := service.ReplaceFileWithMeta(multipartFile(t, "new.PNG", content), "/uploads/product/2026/11/a.png", "product")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if store.replacedURL != "/uploads/product/2026/11/a.png" || !bytes.Equal(store.replaced, content) {
		t.Fatalf("store not called with original url: %q", store.replacedURL)
	}
	if result.URL != "/uploads/product/2026/11/a.png" || result.MimeType != "image/png" || result.Width != 30 || result.Height != 20 || result.Size != int64(len(content)) {
		t.Fatalf("result mismatch: %#v", result)
	}
	if store.data != nil {
		t.Fatal("replace must not create a new file via Save")
	}
}

func TestReplaceFileWithMetaTreatsJPEGAndJPGAsSameFormat(t *testing.T) {
	store := &replacingStore{}
	service := NewService(replacePolicy(), store)
	if _, err := service.ReplaceFileWithMeta(multipartFile(t, "photo.jpeg", encodeImage(t, "jpeg", 4, 4)), "/uploads/common/x.jpg", "common"); err != nil {
		t.Fatalf("jpeg -> jpg should be allowed: %v", err)
	}
}

func TestReplaceFileWithMetaRejectsFormatChanges(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  []byte
		want     string
	}{
		{name: "different extension", filename: "new.jpg", content: encodeImage(t, "jpeg", 4, 4), want: "不一致"},
		{name: "renamed jpeg", filename: "new.png", content: encodeImage(t, "jpeg", 4, 4), want: "不一致"},
		{name: "too large", filename: "new.png", content: bytes.Repeat([]byte("a"), 2<<20), want: "文件大小超过限制"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &replacingStore{}
			service := NewService(replacePolicy(), store)
			_, err := service.ReplaceFileWithMeta(multipartFile(t, test.filename, test.content), "/uploads/common/a.png", "common")
			var validation *contract.ValidationError
			if !errors.As(err, &validation) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want validation error containing %q, got %v", test.want, err)
			}
			if store.replaced != nil {
				t.Fatal("store must not be written on validation failure")
			}
		})
	}
}

func TestReplaceFileWithMetaRequiresReplacerStore(t *testing.T) {
	service := NewService(replacePolicy(), &memoryStore{})
	if _, err := service.ReplaceFileWithMeta(multipartFile(t, "a.png", encodeImage(t, "png", 2, 2)), "/uploads/common/a.png", "common"); err == nil {
		t.Fatal("expected error when store cannot replace")
	}
}
