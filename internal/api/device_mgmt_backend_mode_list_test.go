package api

import (
	"encoding/json"
	"testing"
)

// 设备页靠 backend_mode 区分读卡器与模组（PC/SC 没有射频、数据连接与 AT/选网/eSIM）。
// 该字段此前只存在于 overview 结构体，列表接口漏了，前端拿到的恒为 undefined。
func TestDeviceMgmtListItemCarriesBackendMode(t *testing.T) {
	item := deviceMgmtListItem{ID: "AK9563", Name: "reader", BackendMode: "pcsc"}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	got, ok := decoded["backend_mode"]
	if !ok {
		t.Fatalf("列表项缺少 backend_mode 字段: %s", raw)
	}
	if got != "pcsc" {
		t.Fatalf("backend_mode = %v, want pcsc", got)
	}
}

// 模组设备的模式同样要带出去，前端才能在需要时区分 AT/QMI/MBIM。
func TestDeviceMgmtListItemOmitsEmptyBackendMode(t *testing.T) {
	raw, _ := json.Marshal(deviceMgmtListItem{ID: "x"})
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if _, ok := decoded["backend_mode"]; ok {
		t.Fatalf("空值应被 omitempty 省略: %s", raw)
	}
}
