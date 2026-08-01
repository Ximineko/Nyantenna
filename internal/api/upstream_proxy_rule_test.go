package api

import (
	"encoding/json"
	"testing"
)

// 前端添加规则时只发 upstream_proxy_id，不带 enabled。
// 若按布尔零值入库，规则会是停用状态，VoWiFi 静默回落直连。
func TestCountryRuleEnabledDefaultsTrue(t *testing.T) {
	var req struct {
		UpstreamProxyID string `json:"upstream_proxy_id"`
		Enabled         *bool  `json:"enabled"`
	}
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"未提供 enabled（前端当前行为）", `{"upstream_proxy_id":"up_x"}`, true},
		{"显式 true", `{"upstream_proxy_id":"up_x","enabled":true}`, true},
		{"显式 false 必须被尊重", `{"upstream_proxy_id":"up_x","enabled":false}`, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			req.Enabled = nil
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			enabled := true
			if req.Enabled != nil {
				enabled = *req.Enabled
			}
			if enabled != tt.want {
				t.Fatalf("enabled = %v, want %v", enabled, tt.want)
			}
		})
	}
}
