package config_test

import (
	"testing"

	"github.com/zhaoyta/stealthcopilot/internal/config"
)

// TestKeyringStore_SetGet 验证基本写入读取流程。
func TestKeyringStore_SetGet(t *testing.T) {
	store := config.NewKeyringStore()
	const key = "test_key_set_get"

	if err := store.Set(key, "hello"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != "hello" {
		t.Errorf("want 'hello', got %q", v)
	}
	// 清理
	_ = store.Delete(key)
}

// TestKeyringStore_NotFound 验证 key 不存在时返回 ErrNotFound。
func TestKeyringStore_NotFound(t *testing.T) {
	store := config.NewKeyringStore()
	_, err := store.Get("nonexistent_key_xyz_987654")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

// TestKeyringStore_Delete 验证删除后 key 不可读。
func TestKeyringStore_Delete(t *testing.T) {
	store := config.NewKeyringStore()
	const key = "test_key_delete"

	if err := store.Set(key, "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := store.Get(key)
	if err == nil {
		t.Fatal("expected ErrNotFound after delete, got nil")
	}
}

// TestKeyringStore_SetEmpty 验证写入空值等效于删除（幂等）。
func TestKeyringStore_SetEmpty(t *testing.T) {
	store := config.NewKeyringStore()
	const key = "test_key_empty"

	_ = store.Set(key, "value")
	if err := store.Set(key, ""); err != nil {
		t.Fatalf("Set empty: %v", err)
	}
	_, err := store.Get(key)
	if err == nil {
		t.Error("expected ErrNotFound after Set(''), got nil")
	}
}
