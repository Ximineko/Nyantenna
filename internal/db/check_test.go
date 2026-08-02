package db

import (
	"path/filepath"
	"testing"
)

// TestInitCreatesSchema 用临时目录里的全新库验证 Init 能建库并迁移出核心表。
// （此文件之前是硬编码绝对路径的调试残留，在任何机器上都会 panic。）
func TestInitCreatesSchema(t *testing.T) {
	if err := Init(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	var cols []map[string]any
	if err := DB.Raw("PRAGMA table_info(devices)").Scan(&cols).Error; err != nil {
		t.Fatalf("查询 devices 表结构失败: %v", err)
	}
	if len(cols) == 0 {
		t.Fatal("devices 表不存在或没有列，AutoMigrate 未生效")
	}
}
