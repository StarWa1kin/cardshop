package localstore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/dujiao-next/internal/modules/upload/contract"
)

func TestStoreSavesFileUnderConfiguredRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")
	store := New(root)
	url, err := store.Save(contract.StoreInput{
		Source:   bytes.NewBufferString("asset-content"),
		Scene:    "common",
		Year:     "2026",
		Month:    "07",
		Filename: "asset.txt",
	})
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if url != "/uploads/common/2026/07/asset.txt" {
		t.Fatalf("public URL got %q", url)
	}
	data, err := os.ReadFile(filepath.Join(root, "common", "2026", "07", "asset.txt"))
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(data) != "asset-content" {
		t.Fatalf("saved content got %q", data)
	}
}

func TestStoreReplaceOverwritesExistingFileInPlace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")
	store := New(root)
	url, err := store.Save(contract.StoreInput{
		Source: bytes.NewBufferString("old"), Scene: "common", Year: "2026", Month: "11", Filename: "a.png",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.Replace(url, bytes.NewBufferString("new-content")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "common", "2026", "11", "a.png"))
	if err != nil || string(data) != "new-content" {
		t.Fatalf("replaced content got %q err=%v", data, err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(root, "common", "2026", "11", ".replace-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left: %v", leftovers)
	}
}

func TestStoreReplaceRejectsMissingAndOutsideFiles(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "uploads")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "config.yml"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := New(root)
	for _, url := range []string{"/uploads/common/missing.png", "/uploads/../config.yml", "/config.yml", ""} {
		if err := store.Replace(url, bytes.NewBufferString("x")); err == nil {
			t.Fatalf("replace %q should fail", url)
		}
	}
	data, _ := os.ReadFile(filepath.Join(base, "config.yml"))
	if string(data) != "secret" {
		t.Fatal("file outside uploads root was modified")
	}
	if _, err := os.Stat(filepath.Join(root, "common", "missing.png")); !os.IsNotExist(err) {
		t.Fatal("replace must not create new files")
	}
}
