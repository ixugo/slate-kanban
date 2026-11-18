package blobfs_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"slate-kanban/internal/core/kanban/stores/blobfs"
)

// TestSaveReadRemove 写入后能读回同样的内容，上级目录自动创建，删除后再读得到不存在，重复删除也成功。
func TestSaveReadRemove(t *testing.T) {
	root := t.TempDir()
	d := blobfs.New(root)
	data := []byte{1, 2, 3}
	if err := d.Save("SLATE/a.png", data); err != nil {
		t.Fatal(err)
	}
	if got, err := d.Read("SLATE/a.png"); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("读回 %v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(root, "SLATE", "a.png.tmp")); err == nil {
		t.Fatal("不应留下临时文件")
	}
	if err := d.Remove("SLATE/a.png"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Read("SLATE/a.png"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除后应该读不到，得到 %v", err)
	}
	if err := d.Remove("SLATE/a.png"); err != nil {
		t.Fatalf("重复删除应该成功: %v", err)
	}
}

// TestRejectsEscapingPaths 空、绝对、带 .. 的路径都要拒绝，不能读写根目录之外。
func TestRejectsEscapingPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "res")
	d := blobfs.New(root)
	for _, rel := range []string{"", "/etc/passwd", "../x.png", "a/../../x.png", "..", ".", `a\..\x`} {
		if err := d.Save(rel, []byte{1}); err == nil {
			t.Errorf("Save(%q) 应该被拒绝", rel)
		}
		if _, err := d.Read(rel); err == nil {
			t.Errorf("Read(%q) 应该被拒绝", rel)
		}
		if err := d.Remove(rel); err == nil {
			t.Errorf("Remove(%q) 应该被拒绝", rel)
		}
	}
	if _, err := os.Stat(filepath.Dir(root) + "/x.png"); err == nil {
		t.Fatal("文件被写到了根目录之外")
	}
}

// TestRemoveDir 只删一层子目录；根目录、多层路径和空名字都拒绝，目录不存在算成功。
func TestRemoveDir(t *testing.T) {
	root := t.TempDir()
	d := blobfs.New(root)
	if err := d.Save("A1/a.png", []byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := d.Save("B1/b.png", []byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := d.RemoveDir("A1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "A1")); err == nil {
		t.Fatal("A1 应该被删除")
	}
	if _, err := os.Stat(filepath.Join(root, "B1", "b.png")); err != nil {
		t.Fatal("B1 不应受影响")
	}
	for _, bad := range []string{"", ".", "..", "B1/x", "/B1"} {
		if err := d.RemoveDir(bad); err == nil {
			t.Errorf("RemoveDir(%q) 应该被拒绝", bad)
		}
	}
	if err := d.RemoveDir("NOPE"); err != nil {
		t.Fatalf("目录不存在应该成功: %v", err)
	}
}
