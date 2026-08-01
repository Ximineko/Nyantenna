package api

import (
	"strings"
	"testing"
)

// 前端表单里没有 ID 字段，创建时必须由服务端补齐，
// 否则新建前置代理会以 "id 不能为空" 失败。
func TestNewUpstreamProxyID(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id := newUpstreamProxyID()
		if !strings.HasPrefix(id, "up_") {
			t.Fatalf("ID 缺少 up_ 前缀: %q", id)
		}
		if id == "up_" {
			t.Fatal("生成了空 ID")
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("ID 重复: %q", id)
		}
		seen[id] = struct{}{}
	}
}
