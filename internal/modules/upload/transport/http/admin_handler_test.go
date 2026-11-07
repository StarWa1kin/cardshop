package uploadhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	uploadcontract "github.com/dujiao-next/internal/modules/upload/contract"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type recordingMediaRecorder struct {
	ctx    context.Context
	result contentapp.UploadResult
	scene  string
	media  *contentdomain.Media
	err    error
}

var _ MediaRecorder = (*recordingMediaRecorder)(nil)

func (r *recordingMediaRecorder) RecordMedia(ctx context.Context, result contentapp.UploadResult, scene string) (*contentdomain.Media, error) {
	r.ctx = ctx
	r.result = result
	r.scene = scene
	return r.media, r.err
}

type fakeValidationError struct {
	msg string
}

func (e *fakeValidationError) Error() string          { return e.msg }
func (e *fakeValidationError) UploadValidationError() {}

type fakeUploader struct {
	result *uploadcontract.Result
	err    error
	file   *multipart.FileHeader
	scene  string
}

func (u *fakeUploader) SaveFileWithMeta(file *multipart.FileHeader, scene string) (*uploadcontract.Result, error) {
	u.file = file
	u.scene = scene
	return u.result, u.err
}

func TestUploadFileReturnsValidationMessageForOversizedFile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "oversized.png")
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err := part.Write([]byte("oversized")); err != nil {
		t.Fatalf("write form file failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}

	handler := NewAdminHandler(&fakeUploader{
		err: &fakeValidationError{msg: "文件大小超过限制: 9 / 4 bytes"},
	}, &recordingMediaRecorder{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/upload", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	handler.UploadFile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("http status want 200 got %d body=%s", w.Code, w.Body.String())
	}
	var got response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
	}
	if got.StatusCode != response.CodeBadRequest {
		t.Fatalf("business status want 400 got %d body=%s", got.StatusCode, w.Body.String())
	}
	if !strings.Contains(got.Msg, "文件大小超过限制") {
		t.Fatalf("message should include validation reason, got %q", got.Msg)
	}
}

func TestUploadFileUsesInjectedMediaRecorderAndRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "asset.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("content asset")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.WriteField("scene", "common"); err != nil {
		t.Fatalf("write scene: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	media := &contentdomain.Media{}
	media.ID = 91
	recorder := &recordingMediaRecorder{media: media}
	uploader := &fakeUploader{result: &uploadcontract.Result{
		URL:      "/uploads/common/asset.txt",
		Filename: "asset.txt",
		MimeType: "text/plain",
		Size:     13,
	}}
	handler := NewAdminHandler(uploader, recorder)

	type contextKey struct{}
	requestContext := context.WithValue(context.Background(), contextKey{}, "upload-request")
	request := httptest.NewRequest(http.MethodPost, "/admin/upload", body).WithContext(requestContext)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = request

	handler.UploadFile(c)

	if uploader.scene != "common" || uploader.file == nil || uploader.file.Filename != "asset.txt" {
		t.Fatalf("uploader input mismatch: scene=%q file=%#v", uploader.scene, uploader.file)
	}
	if recorder.ctx.Value(contextKey{}) != "upload-request" {
		t.Fatal("request context was not propagated")
	}
	if recorder.scene != "common" || recorder.result.Filename != "asset.txt" || recorder.result.URL == "" {
		t.Fatalf("media recorder input mismatch: scene=%q result=%#v", recorder.scene, recorder.result)
	}
	var got response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := got.Data.(map[string]interface{})
	if !ok || uint(data["media_id"].(float64)) != media.ID {
		t.Fatalf("response media_id mismatch: %#v", got.Data)
	}
}

func TestUploadFileKeepsUploadSuccessWhenMediaRecordingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "asset.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("content asset")); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	recorder := &recordingMediaRecorder{err: context.Canceled}
	handler := NewAdminHandler(&fakeUploader{result: &uploadcontract.Result{
		URL:      "/uploads/common/asset.txt",
		Filename: "asset.txt",
		Size:     13,
	}}, recorder)
	request := httptest.NewRequest(http.MethodPost, "/admin/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = request

	handler.UploadFile(c)

	var got response.Response
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.StatusCode != response.CodeOK {
		t.Fatalf("media recording failure changed upload result: %#v", got)
	}
	data, ok := got.Data.(map[string]interface{})
	if !ok || uint(data["media_id"].(float64)) != 0 || data["url"] == "" {
		t.Fatalf("best-effort response mismatch: %#v", got.Data)
	}
}

func TestIsUploadValidationErrorRecognizesMarker(t *testing.T) {
	if !isUploadValidationError(&fakeValidationError{msg: "bad"}) {
		t.Fatal("expected validation marker to be recognized")
	}
	if isUploadValidationError(errors.New("other")) {
		t.Fatal("unexpected match for plain error")
	}
}

type fakeFileReplacer struct {
	result    *uploadcontract.Result
	err       error
	gotURL    string
	gotScene  string
	callCount int
}

func (f *fakeFileReplacer) ReplaceFileWithMeta(_ *multipart.FileHeader, publicURL, scene string) (*uploadcontract.Result, error) {
	f.callCount++
	f.gotURL = publicURL
	f.gotScene = scene
	return f.result, f.err
}

type fakeReplacementRecorder struct {
	media    *contentdomain.Media
	getErr   error
	recorded *contentapp.UploadResult
}

func (f *fakeReplacementRecorder) Get(context.Context, uint) (*contentdomain.Media, error) {
	return f.media, f.getErr
}

func (f *fakeReplacementRecorder) RecordReplacement(_ context.Context, _ uint, result contentapp.UploadResult) (*contentdomain.Media, error) {
	f.recorded = &result
	updated := *f.media
	updated.Size = result.Size
	updated.Width = result.Width
	updated.Height = result.Height
	updated.MimeType = result.MimeType
	return &updated, nil
}

func performReplace(t *testing.T, handler *MediaReplaceHandler, id string, withFile bool) response.Response {
	t.Helper()
	gin.SetMode(gin.TestMode)
	body := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(body)
	if withFile {
		part, err := writer.CreateFormFile("file", "new.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte("png-bytes"))
	}
	_ = writer.Close()

	router := gin.New()
	RegisterAdminMediaReplaceRoutes(router.Group("/api/v1/admin"), handler)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/media/"+id+"/file", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var got response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	return got
}

func sampleMedia() *contentdomain.Media {
	media := &contentdomain.Media{Path: "/uploads/product/2026/11/a.png", Scene: "product", MimeType: "image/png", Size: 10}
	media.ID = 4
	return media
}

func TestReplaceMediaFileReplacesAtOriginalURL(t *testing.T) {
	files := &fakeFileReplacer{result: &uploadcontract.Result{URL: "/uploads/product/2026/11/a.png", Filename: "new.png", MimeType: "image/png", Size: 9, Width: 8, Height: 6}}
	recorder := &fakeReplacementRecorder{media: sampleMedia()}

	got := performReplace(t, NewMediaReplaceHandler(files, recorder), "4", true)
	if got.StatusCode != response.CodeOK {
		t.Fatalf("replace failed: %#v", got)
	}
	if files.gotURL != "/uploads/product/2026/11/a.png" || files.gotScene != "product" {
		t.Fatalf("replace must target the existing path and scene: %q %q", files.gotURL, files.gotScene)
	}
	if recorder.recorded == nil || recorder.recorded.Size != 9 || recorder.recorded.Width != 8 {
		t.Fatalf("metadata not recorded: %#v", recorder.recorded)
	}
	data := got.Data.(map[string]interface{})
	if data["path"] != "/uploads/product/2026/11/a.png" || data["size"].(float64) != 9 {
		t.Fatalf("response mismatch: %#v", data)
	}
}

func TestReplaceMediaFileErrors(t *testing.T) {
	t.Run("validation error is shown", func(t *testing.T) {
		files := &fakeFileReplacer{err: &uploadcontract.ValidationError{Message: "新文件格式（.jpg）与原素材（.png）不一致"}}
		recorder := &fakeReplacementRecorder{media: sampleMedia()}
		got := performReplace(t, NewMediaReplaceHandler(files, recorder), "4", true)
		if got.StatusCode != response.CodeBadRequest || got.Msg != "新文件格式（.jpg）与原素材（.png）不一致" || recorder.recorded != nil {
			t.Fatalf("unexpected: %#v", got)
		}
	})
	t.Run("media not found", func(t *testing.T) {
		files := &fakeFileReplacer{}
		got := performReplace(t, NewMediaReplaceHandler(files, &fakeReplacementRecorder{getErr: contentcontract.ErrMediaNotFound}), "4", true)
		if got.StatusCode != response.CodeNotFound || files.callCount != 0 {
			t.Fatalf("unexpected: %#v", got)
		}
	})
	t.Run("file missing on disk", func(t *testing.T) {
		files := &fakeFileReplacer{err: fs.ErrNotExist}
		got := performReplace(t, NewMediaReplaceHandler(files, &fakeReplacementRecorder{media: sampleMedia()}), "4", true)
		if got.StatusCode != response.CodeNotFound {
			t.Fatalf("unexpected: %#v", got)
		}
	})
	t.Run("no file in request", func(t *testing.T) {
		files := &fakeFileReplacer{}
		got := performReplace(t, NewMediaReplaceHandler(files, &fakeReplacementRecorder{media: sampleMedia()}), "4", false)
		if got.StatusCode != response.CodeBadRequest || files.callCount != 0 {
			t.Fatalf("unexpected: %#v", got)
		}
	})
	t.Run("invalid id", func(t *testing.T) {
		files := &fakeFileReplacer{}
		got := performReplace(t, NewMediaReplaceHandler(files, &fakeReplacementRecorder{media: sampleMedia()}), "abc", true)
		if got.StatusCode != response.CodeBadRequest || files.callCount != 0 {
			t.Fatalf("unexpected: %#v", got)
		}
	})
}
