package localstore

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dujiao-next/internal/modules/upload/contract"
)

// Store 将上传文件写入本地 uploads 目录。
type Store struct {
	root string
}

var _ contract.Store = (*Store)(nil)
var _ contract.Replacer = (*Store)(nil)

// New 创建本地文件存储适配器。
func New(root string) *Store {
	if root == "" {
		panic("upload local store: root is empty")
	}
	return &Store{root: root}
}

// Save 保存文件并返回公开访问 URL。
func (s *Store) Save(input contract.StoreInput) (string, error) {
	directory := filepath.Join(s.root, input.Scene, input.Year, input.Month)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(directory, input.Filename)
	destination, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(destination, input.Source); err != nil {
		_ = destination.Close()
		return "", err
	}
	if err := destination.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("/uploads/%s/%s/%s/%s", input.Scene, input.Year, input.Month, input.Filename), nil
}

// Replace 按公开 URL 原地覆盖已有文件：先写同目录临时文件再重命名，避免读到写了一半的文件。
// 只允许覆盖 uploads 目录下已经存在的文件。
func (s *Store) Replace(publicURL string, source io.Reader) error {
	target, err := s.resolvePublicURL(publicURL)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("upload local store: %s is not a regular file", publicURL)
	}

	temp, err := os.CreateTemp(filepath.Dir(target), ".replace-*"+filepath.Ext(target))
	if err != nil {
		return err
	}
	tempName := temp.Name()
	fail := func(err error) error {
		_ = os.Remove(tempName)
		return err
	}
	if _, err := io.Copy(temp, source); err != nil {
		_ = temp.Close()
		return fail(err)
	}
	if err := temp.Close(); err != nil {
		return fail(err)
	}
	if err := os.Chmod(tempName, info.Mode().Perm()); err != nil {
		return fail(err)
	}
	if err := os.Rename(tempName, target); err != nil {
		return fail(err)
	}
	return nil
}

func (s *Store) resolvePublicURL(publicURL string) (string, error) {
	cleaned := path.Clean("/" + strings.TrimSpace(publicURL))
	if !strings.HasPrefix(cleaned, "/uploads/") {
		return "", fmt.Errorf("upload local store: invalid public url %q", publicURL)
	}
	relative := filepath.FromSlash(strings.TrimPrefix(cleaned, "/uploads/"))
	target := filepath.Join(s.root, relative)
	within, err := filepath.Rel(s.root, target)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("upload local store: invalid public url %q", publicURL)
	}
	return target, nil
}
