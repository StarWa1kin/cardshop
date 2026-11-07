package application

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
)

type compressMediaStore struct {
	media   *domain.Media
	updated *domain.Media
}

var _ contract.MediaStore = (*compressMediaStore)(nil)

func (s *compressMediaStore) List(context.Context, contract.MediaQuery) ([]domain.Media, int64, error) {
	return nil, 0, nil
}
func (s *compressMediaStore) GetByID(context.Context, uint) (*domain.Media, error) {
	return s.media, nil
}
func (s *compressMediaStore) GetByPath(context.Context, string) (*domain.Media, error) {
	return nil, nil
}
func (s *compressMediaStore) Create(context.Context, *domain.Media) error { return nil }
func (s *compressMediaStore) Update(_ context.Context, media *domain.Media) error {
	copied := *media
	s.updated = &copied
	return nil
}
func (s *compressMediaStore) Delete(context.Context, uint) error { return nil }

type memoryReplacer struct {
	files    map[string][]byte
	replaced map[string][]byte
}

func (m *memoryReplacer) ReadFile(name string) ([]byte, error) {
	data, ok := m.files[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return data, nil
}

func (m *memoryReplacer) ReplaceFile(name string, data []byte) error {
	if m.replaced == nil {
		m.replaced = map[string][]byte{}
	}
	m.replaced[name] = data
	m.files[name] = data
	return nil
}

type fakeCompressor struct {
	output *contract.CompressedImage
	err    error
	calls  int
	gotKey string
}

func (f *fakeCompressor) Compress(_ context.Context, apiKey string, _ []byte) (*contract.CompressedImage, error) {
	f.calls++
	f.gotKey = apiKey
	return f.output, f.err
}

type staticKey string

func (k staticKey) GetTinyPNGAPIKey() (string, error) { return string(k), nil }

func testPNG(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 7), G: uint8(y * 13), B: uint8(x ^ y), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buffer.Bytes()
}

func newCompressFixture(t *testing.T, path string, original []byte, compressor *fakeCompressor, key string) (*MediaCompressService, *compressMediaStore, *memoryReplacer) {
	t.Helper()
	media := &domain.Media{Path: path, Size: int64(len(original)), MimeType: "image/png"}
	media.ID = 7
	store := &compressMediaStore{media: media}
	files := &memoryReplacer{files: map[string][]byte{}}
	if original != nil {
		files.files[path[1:]] = original
	}
	service := NewMediaCompressService(store, files, compressor, staticKey(key))
	service.now = func() time.Time { return time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC) }
	return service, store, files
}

func TestMediaCompressReplacesFileInPlaceAndUpdatesSize(t *testing.T) {
	original := testPNG(t, 64)
	smaller := testPNG(t, 2)
	compressor := &fakeCompressor{output: &contract.CompressedImage{Data: smaller, MimeType: "image/png", CompressionCount: 12}}
	service, store, files := newCompressFixture(t, "/uploads/editor/2026/11/a.png", original, compressor, "test-key")

	result, err := service.Compress(context.Background(), 7)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if compressor.gotKey != "test-key" {
		t.Fatalf("api key not passed: %q", compressor.gotKey)
	}
	if !bytes.Equal(files.replaced["uploads/editor/2026/11/a.png"], smaller) {
		t.Fatal("file was not replaced at the original path")
	}
	if !result.Replaced || result.BeforeSize != int64(len(original)) || result.AfterSize != int64(len(smaller)) {
		t.Fatalf("result sizes mismatch: %#v", result)
	}
	if result.SavedBytes != result.BeforeSize-result.AfterSize || result.SavedPercent <= 0 || result.SavedPercent >= 100 {
		t.Fatalf("saved stats mismatch: %#v", result)
	}
	if result.CompressionCount != 12 {
		t.Fatalf("compression count mismatch: %d", result.CompressionCount)
	}
	if store.updated == nil || store.updated.Size != int64(len(smaller)) || store.updated.OriginalSize != int64(len(original)) || store.updated.CompressedAt == nil {
		t.Fatalf("media record not updated: %#v", store.updated)
	}
	if store.updated.Path != "/uploads/editor/2026/11/a.png" {
		t.Fatalf("media path must stay unchanged, got %q", store.updated.Path)
	}
}

func TestMediaCompressKeepsOriginalWhenResultIsNotSmaller(t *testing.T) {
	original := testPNG(t, 2)
	compressor := &fakeCompressor{output: &contract.CompressedImage{Data: testPNG(t, 64)}}
	service, store, files := newCompressFixture(t, "/uploads/common/b.png", original, compressor, "k")

	result, err := service.Compress(context.Background(), 7)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if result.Replaced || result.SavedBytes != 0 || result.SavedPercent != 0 || result.AfterSize != result.BeforeSize {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(files.replaced) != 0 {
		t.Fatal("original file must not be replaced")
	}
	if store.updated == nil || store.updated.CompressedAt == nil {
		t.Fatal("media should still be marked as compressed")
	}
}

func TestMediaCompressRejectsUnsupportedFormatsAndMissingKey(t *testing.T) {
	png := testPNG(t, 4)
	tests := []struct {
		name    string
		path    string
		content []byte
		key     string
		want    error
	}{
		{name: "gif extension", path: "/uploads/common/a.gif", content: []byte("GIF89a..."), key: "k", want: contract.ErrCompressUnsupportedFormat},
		{name: "svg extension", path: "/uploads/common/a.svg", content: []byte("<svg/>"), key: "k", want: contract.ErrCompressUnsupportedFormat},
		{name: "renamed gif", path: "/uploads/common/a.png", content: []byte("GIF89a\x01\x00\x01\x00"), key: "k", want: contract.ErrCompressUnsupportedFormat},
		{name: "missing key", path: "/uploads/common/a.png", content: png, key: "  ", want: contract.ErrCompressKeyMissing},
		{name: "missing file", path: "/uploads/common/a.png", content: nil, key: "k", want: contract.ErrMediaFileMissing},
		{name: "path traversal", path: "/uploads/../config.png", content: png, key: "k", want: contract.ErrMediaFileMissing},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compressor := &fakeCompressor{output: &contract.CompressedImage{Data: png}}
			service, _, _ := newCompressFixture(t, test.path, test.content, compressor, test.key)
			_, err := service.Compress(context.Background(), 7)
			if !errors.Is(err, test.want) {
				t.Fatalf("want %v got %v", test.want, err)
			}
			if compressor.calls != 0 {
				t.Fatal("compressor must not be called")
			}
		})
	}
}

func TestMediaCompressRejectsOutputWithDifferentFormat(t *testing.T) {
	compressor := &fakeCompressor{output: &contract.CompressedImage{Data: []byte("\xff\xd8\xff\xe0 small jpeg")}}
	service, store, files := newCompressFixture(t, "/uploads/common/c.png", testPNG(t, 32), compressor, "k")
	_, err := service.Compress(context.Background(), 7)
	if !errors.Is(err, contract.ErrCompressFailed) {
		t.Fatalf("want ErrCompressFailed got %v", err)
	}
	if len(files.replaced) != 0 || store.updated != nil {
		t.Fatal("nothing should be written when output format differs")
	}
}

func TestMediaCompressPropagatesCompressorErrors(t *testing.T) {
	compressor := &fakeCompressor{err: contract.ErrCompressQuotaExceeded}
	service, store, _ := newCompressFixture(t, "/uploads/common/d.png", testPNG(t, 8), compressor, "k")
	if _, err := service.Compress(context.Background(), 7); !errors.Is(err, contract.ErrCompressQuotaExceeded) {
		t.Fatalf("want quota error got %v", err)
	}
	if store.updated != nil {
		t.Fatal("media must not be updated on failure")
	}
}

func TestMediaCompressReturnsNotFound(t *testing.T) {
	store := &compressMediaStore{}
	service := NewMediaCompressService(store, &memoryReplacer{files: map[string][]byte{}}, &fakeCompressor{}, staticKey("k"))
	if _, err := service.Compress(context.Background(), 1); !errors.Is(err, contract.ErrMediaNotFound) {
		t.Fatalf("want not found got %v", err)
	}
}

func TestMediaRecordReplacementResetsCompressionState(t *testing.T) {
	compressedAt := time.Now()
	media := &domain.Media{Path: "/uploads/common/a.png", Name: "banner", Filename: "a.png", MimeType: "image/png", Size: 500, Width: 10, Height: 10, OriginalSize: 900, CompressedAt: &compressedAt}
	media.ID = 3
	store := &compressMediaStore{media: media}
	service := NewMediaService(store, nil, nil)

	updated, err := service.RecordReplacement(context.Background(), 3, UploadResult{URL: "/uploads/common/a.png", Filename: "new.png", MimeType: "image/png", Size: 1234, Width: 40, Height: 30})
	if err != nil {
		t.Fatalf("record replacement: %v", err)
	}
	if updated.Size != 1234 || updated.Width != 40 || updated.Height != 30 || updated.OriginalSize != 0 || updated.CompressedAt != nil {
		t.Fatalf("metadata not updated: %#v", updated)
	}
	if updated.Path != "/uploads/common/a.png" || updated.Name != "banner" || updated.Filename != "a.png" {
		t.Fatalf("path/name must stay unchanged: %#v", updated)
	}
	if store.updated == nil || store.updated.Size != 1234 {
		t.Fatal("store was not updated")
	}

	missing := NewMediaService(&compressMediaStore{}, nil, nil)
	if _, err := missing.RecordReplacement(context.Background(), 1, UploadResult{}); !errors.Is(err, contract.ErrMediaNotFound) {
		t.Fatalf("want not found got %v", err)
	}
}
