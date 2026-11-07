package local

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dujiao-next/internal/modules/content/contract"
)

// Store 使用当前工作目录下的本地文件系统。
type Store struct{}

var _ contract.FileStore = Store{}
var _ contract.MediaFileReplacer = Store{}

// New 创建本地文件适配器。
func New() Store {
	return Store{}
}

// Stat 返回文件信息。
func (Store) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// Open 打开文件用于读取。
func (Store) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

// Remove 删除文件。
func (Store) Remove(name string) error {
	return os.Remove(name)
}

// ReadFile 读取整个文件。
func (Store) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// ReplaceFile 先写入同目录临时文件再重命名覆盖，避免写到一半时线上读到残缺图片。
func (Store) ReplaceFile(name string, data []byte) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(name), ".compress-*"+filepath.Ext(name))
	if err != nil {
		return err
	}
	tempName := temp.Name()
	cleanup := func() { _ = os.Remove(tempName) }
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		cleanup()
		return err
	}
	if err := temp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tempName, info.Mode().Perm()); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tempName, name); err != nil {
		cleanup()
		return err
	}
	return nil
}
