package http_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	localfilestore "github.com/dujiao-next/internal/modules/content/infrastructure/filestore/local"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	"github.com/dujiao-next/internal/modules/content/infrastructure/tinify"
	contenttransport "github.com/dujiao-next/internal/modules/content/transport/http"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type staticTinyPNGKey string

func (k staticTinyPNGKey) GetTinyPNGAPIKey() (string, error) { return string(k), nil }

func encodeNoisyPNG(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 5), G: uint8(y * 3), B: uint8(x ^ y), A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// 端到端：真实 SQLite 素材记录 + 本地磁盘文件 + TinyPNG 客户端（指向 mock 服务，不调用真实 API）。
func TestCompressMediaEndToEndWithMockTinyPNG(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	_, _, db := setupAdminContentHandlerTest(t)

	original := encodeNoisyPNG(t, 48)
	compressed := encodeNoisyPNG(t, 2)
	relative := filepath.FromSlash("uploads/editor/2026/11/pic.png")
	if err := os.MkdirAll(filepath.Dir(relative), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(relative, original, 0o644); err != nil {
		t.Fatal(err)
	}
	media := contentdomain.Media{
		Name: "pic", Filename: "pic.png", Path: "/uploads/editor/2026/11/pic.png",
		MimeType: "image/png", Size: int64(len(original)), Scene: "editor", Width: 48, Height: 48,
	}
	if err := db.Create(&media).Error; err != nil {
		t.Fatal(err)
	}

	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/shrink", func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "api" || pass != "mock-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, original) {
			t.Errorf("uploaded bytes differ from the file on disk")
		}
		w.Header().Set("Location", server.URL+"/output/1")
		w.Header().Set("Compression-Count", "3")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"output":{"type":"image/png"}}`)
	})
	mux.HandleFunc("/output/1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(compressed)
	})
	server = httptest.NewServer(mux)
	defer server.Close()

	client, err := tinify.New(tinify.Options{Endpoint: server.URL + "/shrink", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	service := contentapp.NewMediaCompressService(gormstore.NewMediaStore(db), localfilestore.New(), client, staticTinyPNGKey("mock-key"))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	contenttransport.RegisterAdminMediaCompressRoutes(router.Group("/api/v1/admin"), contenttransport.NewMediaCompressHandler(service))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/admin/media/"+strconv.FormatUint(uint64(media.ID), 10)+"/compress", nil))

	var got struct {
		StatusCode int                    `json:"status_code"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if got.StatusCode != response.CodeOK {
		t.Fatalf("compress failed: %s", recorder.Body.String())
	}
	if got.Data["before_size"].(float64) != float64(len(original)) || got.Data["after_size"].(float64) != float64(len(compressed)) {
		t.Fatalf("size stats mismatch: %#v", got.Data)
	}
	if got.Data["saved_percent"].(float64) <= 0 || got.Data["replaced"] != true || got.Data["compression_count"].(float64) != 3 {
		t.Fatalf("result mismatch: %#v", got.Data)
	}

	onDisk, err := os.ReadFile(relative)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, compressed) {
		t.Fatal("file on disk was not replaced in place")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(relative), ".compress-*")); len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}

	var stored contentdomain.Media
	if err := db.First(&stored, media.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Size != int64(len(compressed)) || stored.OriginalSize != int64(len(original)) || stored.CompressedAt == nil {
		t.Fatalf("media record not updated: %#v", stored)
	}
	if stored.Path != media.Path {
		t.Fatalf("media path must stay unchanged, got %q", stored.Path)
	}
}
