package device

import (
	"context"
	"fmt"

	"github.com/iniwex5/vowifi-go/runtimehost"
	"github.com/iniwex5/vowifi-go/runtimehost/messaging"
	"github.com/ximineko/nyantenna/pkg/smscodec"
)

func (p *Pool) GetVoWiFiApp() *runtimehost.Instance {
	return p.GetVoWiFiAppForDevice()
}

func (p *Pool) GetVoWiFiAppForDevice(deviceID ...string) *runtimehost.Instance {
	if len(deviceID) > 0 && deviceID[0] != "" {
		return p.voWiFiHost().Instance(deviceID[0])
	} else {
		for _, app := range p.voWiFiHost().Instances() {
			return app
		}
	}

	return nil
}

func (p *Pool) GetAllVoWiFiApps() map[string]*runtimehost.Instance {
	return p.voWiFiHost().Instances()
}

func (p *Pool) SendVoWiFiSMS(ctx context.Context, deviceID, to, text string) error {
	_, err := p.SendVoWiFiSMSWithResult(ctx, deviceID, to, text)
	return err
}

func (p *Pool) SendVoWiFiSMSWithResult(ctx context.Context, deviceID, to, text string) (messaging.SendOutcome, error) {
	return p.SendVoWiFiSMSWithOptions(ctx, deviceID, to, text, smscodec.SubmitOptions{})
}

// 以下几个方法一律调用 Instance 自身的方法，不要改回 inst.Service()。
//
// runtimehost 的 Instance 有两条构造路径：旧的兼容路径在构造时就把服务写进
// Instance.service，而我们实际使用的 StartModeMain 走 core 路径，服务是在
// ipsec_up 事件里挂到 Instance.core 上的，Instance.service 始终为 nil。
// 于是 inst.Service() 恒返回 nil，会把注册完好的链路误判成“IMS 服务未就绪”，
// 发短信与 USSD 全部发不出去（收短信不经此路径，所以看起来只有发的方向坏了）。
// Instance 上的方法会先取 core 适配器、再回退 Service()，两条路径都正确。

func (p *Pool) SendVoWiFiSMSWithOptions(ctx context.Context, deviceID, to, text string, opts smscodec.SubmitOptions) (messaging.SendOutcome, error) {
	inst := p.voWiFiHost().Instance(deviceID)
	if inst == nil {
		return messaging.SendOutcome{}, fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
	}
	return inst.SendSMSWithOptions(ctx, to, text, messaging.SendOptions{Encoding: string(opts.Encoding)})
}

func (p *Pool) IsVoWiFiActive(deviceID string) bool {
	return p.voWiFiHost().Active(deviceID)
}

// SendVoWiFiUSSD 通过 VoWiFi 发送 USSD 请求（首轮）。
func (p *Pool) SendVoWiFiUSSD(ctx context.Context, deviceID, command string) (*messaging.USSDResult, error) {
	inst := p.voWiFiHost().Instance(deviceID)
	if inst == nil {
		return nil, fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
	}
	return inst.SendUSSD(ctx, command)
}

// ContinueVoWiFiUSSD 在已有 VoWiFi USSD 会话中发送后续输入。
func (p *Pool) ContinueVoWiFiUSSD(ctx context.Context, deviceID, sessionID, input string) (*messaging.USSDResult, error) {
	inst := p.voWiFiHost().Instance(deviceID)
	if inst == nil {
		return nil, fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
	}
	return inst.ContinueUSSD(ctx, sessionID, input)
}

// CancelVoWiFiUSSD 取消 VoWiFi USSD 会话。
func (p *Pool) CancelVoWiFiUSSD(ctx context.Context, deviceID, sessionID string) error {
	inst := p.voWiFiHost().Instance(deviceID)
	if inst == nil {
		return fmt.Errorf("设备 %s 的 VoWiFi 未启动", deviceID)
	}
	return inst.CancelUSSD(ctx, sessionID)
}

func (p *Pool) GetVoWiFiStatus() (enabled bool, deviceID string, status string) {
	for devID, inst := range p.voWiFiHost().Instances() {
		if inst == nil {
			return true, devID, "VoWiFi: STOPPED"
		}
		return true, devID, inst.Status()
	}
	return false, "", "未初始化"
}

func (p *Pool) GetVoWiFiStatusAll() map[string]string {
	result := make(map[string]string)
	for devID, inst := range p.voWiFiHost().Instances() {
		result[devID] = inst.Status()
	}
	return result
}

func (p *Pool) GetVoWiFiObs(deviceID string) map[string]interface{} {
	if inst := p.voWiFiHost().Instance(deviceID); inst != nil {
		return inst.Obs()
	}
	return nil
}

func (p *Pool) GetVoWiFiRuntimeState(deviceID string) (runtimehost.State, bool) {
	return p.voWiFiHost().State(deviceID)
}

func (p *Pool) SubscribeVoWiFiState(deviceID string) (<-chan struct{}, func()) {
	return p.voWiFiHost().SubscribeState(deviceID)
}

func (p *Pool) recordVoWiFiStartupState(deviceID string, state runtimehost.State) {
	p.voWiFiHost().RecordStartupState(deviceID, state)
}

func (p *Pool) clearVoWiFiStartupState(deviceID string) {
	p.voWiFiHost().ClearStartupState(deviceID)
}

func (p *Pool) clearVoWiFiStartupStateAndBroadcast(deviceID string) {
	p.voWiFiHost().ClearStartupStateAndBroadcast(deviceID)
}

func (p *Pool) broadcastVoWiFiStateChange(deviceID string) {
	p.voWiFiHost().BroadcastState(deviceID)
}
