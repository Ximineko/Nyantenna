package device

import (
	"context"
	"strings"
	"testing"

	"github.com/iniwex5/vowifi-go/runtimehost"
	"github.com/ximineko/nyantenna/pkg/smscodec"
)

// 我们实际使用的 StartModeMain 走 runtimehost 的 core 路径，服务挂在 Instance.core 上，
// Instance.service 始终为 nil。若这几个入口拿 inst.Service() 当就绪判据，注册完好的链路
// 也会被误报成“IMS 服务未就绪”，发短信和 USSD 全部发不出去。
//
// 从包外造不出 core 路径的 Instance（core 字段不导出），因此这里用零值 Instance 覆盖：
// 它同样让 Service() 返回 nil，足以钉死“不得再以 Service() 为就绪判据”这一条。
func newPoolWithInstance(t *testing.T, deviceID string, inst *runtimehost.Instance) *Pool {
	t.Helper()
	p := &Pool{}
	p.voWiFiHost().RuntimeStore().SetInstance(deviceID, inst)
	return p
}

func TestVoWiFiEntrypointsDoNotGateOnLegacyService(t *testing.T) {
	const deviceID = "dev-vowifi"
	ctx := context.Background()

	tests := []struct {
		name string
		call func(p *Pool) error
	}{
		{"发送短信", func(p *Pool) error {
			_, err := p.SendVoWiFiSMSWithOptions(ctx, deviceID, "10086", "hi", smscodec.SubmitOptions{})
			return err
		}},
		{"发起 USSD", func(p *Pool) error {
			_, err := p.SendVoWiFiUSSD(ctx, deviceID, "*100#")
			return err
		}},
		{"续接 USSD", func(p *Pool) error {
			_, err := p.ContinueVoWiFiUSSD(ctx, deviceID, "sess-1", "1")
			return err
		}},
		{"取消 USSD", func(p *Pool) error {
			return p.CancelVoWiFiUSSD(ctx, deviceID, "sess-1")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newPoolWithInstance(t, deviceID, &runtimehost.Instance{})
			err := tt.call(p)
			if err == nil {
				return // 零值实例本就发不出去，这里只关心失败的理由
			}
			if strings.Contains(err.Error(), "IMS 服务未就绪") {
				t.Fatalf("不得再用 Service() 当就绪判据（core 路径下恒为 nil）: %v", err)
			}
		})
	}
}

// 实例不存在与实例存在但服务不可用是两回事，前者的提示要保留。
func TestVoWiFiEntrypointsReportNotStartedWithoutInstance(t *testing.T) {
	p := &Pool{}
	if _, err := p.SendVoWiFiSMSWithOptions(context.Background(), "missing", "10086", "hi", smscodec.SubmitOptions{}); err == nil ||
		!strings.Contains(err.Error(), "VoWiFi 未启动") {
		t.Fatalf("无实例时应提示未启动，实际: %v", err)
	}
}
