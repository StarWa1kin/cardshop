// Package tinify 通过 TinyPNG HTTP API 压缩图片。
package tinify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/content/contract"
)

const (
	// DefaultEndpoint 是 TinyPNG 的压缩接口地址。
	DefaultEndpoint = "https://api.tinify.com/shrink"
	// DefaultTimeout 覆盖上传、压缩和下载结果的整个过程。
	DefaultTimeout = 60 * time.Second
	// DefaultMaxDownloadBytes 限制下载结果的大小，防止异常响应占满内存。
	DefaultMaxDownloadBytes int64 = 50 << 20
)

// Options 配置 TinyPNG 客户端；零值字段使用默认值。
type Options struct {
	Endpoint         string
	Timeout          time.Duration
	MaxDownloadBytes int64
	HTTPClient       *http.Client
}

// Client 是 TinyPNG API 客户端。
type Client struct {
	endpoint    *url.URL
	http        *http.Client
	maxDownload int64
}

var _ contract.ImageCompressor = (*Client)(nil)

// New 创建 TinyPNG 客户端。
func New(options Options) (*Client, error) {
	endpoint := strings.TrimSpace(options.Endpoint)
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("tinify: invalid endpoint %q", endpoint)
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	maxDownload := options.MaxDownloadBytes
	if maxDownload <= 0 {
		maxDownload = DefaultMaxDownloadBytes
	}
	return &Client{endpoint: parsed, http: httpClient, maxDownload: maxDownload}, nil
}

type shrinkResponse struct {
	Output struct {
		Size int64  `json:"size"`
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"output"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Compress 上传图片到 TinyPNG，并下载压缩结果。
func (c *Client) Compress(ctx context.Context, apiKey string, data []byte) (*contract.CompressedImage, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, contract.ErrCompressKeyMissing
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint.String(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth("api", apiKey)
	request.Header.Set("Content-Type", "application/octet-stream")

	response, err := c.http.Do(request)
	if err != nil {
		return nil, mapTransportError(err)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	_ = response.Body.Close()
	if err != nil {
		return nil, mapTransportError(err)
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusOK {
		return nil, mapStatusError(response.StatusCode, body)
	}

	var shrink shrinkResponse
	if err := json.Unmarshal(body, &shrink); err != nil {
		return nil, fmt.Errorf("%w: invalid response: %v", contract.ErrCompressFailed, err)
	}
	location := strings.TrimSpace(response.Header.Get("Location"))
	if location == "" {
		location = strings.TrimSpace(shrink.Output.URL)
	}
	outputURL, err := c.resolveOutputURL(location)
	if err != nil {
		return nil, err
	}

	output, err := c.download(ctx, apiKey, outputURL)
	if err != nil {
		return nil, err
	}
	count, _ := strconv.Atoi(strings.TrimSpace(response.Header.Get("Compression-Count")))
	return &contract.CompressedImage{
		Data:             output,
		MimeType:         shrink.Output.Type,
		CompressionCount: count,
	}, nil
}

// resolveOutputURL 只允许从与压缩接口同源的地址下载结果，避免把 Key 发给其它主机。
func (c *Client) resolveOutputURL(location string) (*url.URL, error) {
	if location == "" {
		return nil, fmt.Errorf("%w: missing output location", contract.ErrCompressFailed)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid output location", contract.ErrCompressFailed)
	}
	resolved := c.endpoint.ResolveReference(parsed)
	if resolved.Scheme != c.endpoint.Scheme || resolved.Host != c.endpoint.Host {
		return nil, fmt.Errorf("%w: unexpected output host %s", contract.ErrCompressFailed, resolved.Host)
	}
	return resolved, nil
}

func (c *Client) download(ctx context.Context, apiKey string, outputURL *url.URL) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, outputURL.String(), nil)
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth("api", apiKey)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, mapTransportError(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return nil, mapStatusError(response.StatusCode, body)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, c.maxDownload+1))
	if err != nil {
		return nil, mapTransportError(err)
	}
	if int64(len(data)) > c.maxDownload {
		return nil, fmt.Errorf("%w: output exceeds %d bytes", contract.ErrCompressFailed, c.maxDownload)
	}
	return data, nil
}

func mapStatusError(status int, body []byte) error {
	var payload errorResponse
	_ = json.Unmarshal(body, &payload)
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = strings.TrimSpace(payload.Error)
	}
	if message == "" {
		message = http.StatusText(status)
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: %s", contract.ErrCompressUnauthorized, message)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", contract.ErrCompressQuotaExceeded, message)
	case http.StatusUnsupportedMediaType:
		return fmt.Errorf("%w: %s", contract.ErrCompressUnsupportedFormat, message)
	default:
		return fmt.Errorf("%w: HTTP %d: %s", contract.ErrCompressFailed, status, message)
	}
}

func mapTransportError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", contract.ErrCompressTimeout, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%w: %v", contract.ErrCompressTimeout, err)
	}
	return fmt.Errorf("%w: %v", contract.ErrCompressFailed, err)
}
