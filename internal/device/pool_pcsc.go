package device

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ximineko/nyantenna/internal/apduarbiter"
	"github.com/ximineko/nyantenna/internal/backend"
	"github.com/ximineko/nyantenna/internal/config"
	"github.com/ximineko/nyantenna/internal/sim/pcsc"
	"github.com/ximineko/nyantenna/pkg/logger"
	"github.com/ximineko/nyantenna/pkg/smscodec"
)

// isPCSCDeviceConfig 判断配置是否为"仅 PC/SC 读卡器"设备。
func isPCSCDeviceConfig(cfg config.DeviceConfig) bool {
	return resolvedBackendMode(cfg) == backend.BackendPCSC
}

// IsPCSC 表示该 Worker 背后只有一个读卡器，没有基带。
//
// 这类设备没有网卡、没有射频、没有 AT 口：数据连接、信号轮询、
// 蜂窝短信、选网、模组重启一概不适用，调用方需据此跳过。
func (w *Worker) IsPCSC() bool {
	if w == nil || w.Backend == nil {
		return false
	}
	return strings.EqualFold(w.Backend.Mode(), backend.BackendPCSC)
}

// PCSCCard 返回该 Worker 持有的读卡器句柄；非 PC/SC 设备返回 nil。
func (w *Worker) PCSCCard() *pcsc.Card {
	if w == nil {
		return nil
	}
	return w.pcscCard
}

// excludePCSCDevices 从配置列表里剔除 PC/SC 设备。
//
// 所有基于"USB 硬件枚举 + IMEI 比对"的流程都必须先过这一层：PC/SC 设备的 IMEI 是
// 人工填的，硬件侧根本不存在对应模组，混进去会被判成 Offline 而误拆 Worker。
func excludePCSCDevices(devices []config.DeviceConfig) []config.DeviceConfig {
	out := make([]config.DeviceConfig, 0, len(devices))
	for _, cfg := range devices {
		if isPCSCDeviceConfig(cfg) {
			continue
		}
		out = append(out, cfg)
	}
	return out
}

// addPCSCWorker 启动一个仅 PC/SC 的设备。
//
// 完全绕开 AddWorkerFromConfig 的模组发现链路：没有 USB 枚举、没有 QMI/MBIM Core、
// 没有网卡绑定，也不需要按 IMEI 反查 AT 口。要做的只有连上读卡器、读出卡内身份、
// 注册 Worker，然后把卡策略应用上去。
func (p *Pool) addPCSCWorker(devCfg config.DeviceConfig) (*Worker, error) {
	reader := strings.TrimSpace(devCfg.PCSCReader)
	card, err := pcsc.Open(reader)
	if err != nil {
		return nil, fmt.Errorf("连接 PC/SC 读卡器失败: %w", err)
	}

	be := backend.NewPCSCBackend(pcsc.AsBackendCard(card), devCfg.ModemIMEI)

	// 先确认卡真的能读——读不出 IMSI 的读卡器起来了也没有意义，
	// 与其让设备以"不健康"状态挂着，不如启动即失败并把原因说清楚。
	probeCtx, cancel := context.WithTimeout(p.Context(), 10*time.Second)
	imsi, err := be.GetIMSI(probeCtx)
	cancel()
	if err != nil {
		_ = card.Close()
		return nil, fmt.Errorf("读取 SIM 失败（检查卡是否插好、读卡器是否支持 1.8V/3V）: %w", err)
	}

	w := &Worker{
		ID:     devCfg.ID,
		Config: devCfg,
		// Modem / QMICore / MBIMCore 恒为 nil：没有基带可管。
		Backend:     be,
		pcscCard:    card,
		APDUArbiter: apduarbiter.New(devCfg.ID, apduarbiter.Options{MaxLeaseHold: 10 * time.Minute, MaxSessions: 1, MaxQMITransports: 0}),
		Pool:        p,
		stop:        make(chan struct{}),
		reassembler: smscodec.NewReassembler(),
		// 短信只可能来自 IMS：读卡器收不到蜂窝短信，AT/QMI 两条轮询路径都必须关死。
		smsMode: smsModeVoWiFi,
	}
	p.assignWorkerGeneration(w)

	logger.Info("PC/SC 设备已就绪",
		"device", devCfg.ID,
		"reader", card.ID(),
		"imsi_len", len(imsi))

	p.mu.Lock()
	p.workers[devCfg.ID] = w
	p.mu.Unlock()

	// 身份与卡策略：ICCID 是卡策略的主键，必须在这一步落定。
	go func(worker *Worker) {
		if _, err := p.refreshIdentityAndApplyCardPolicy(worker, "pcsc_startup"); err != nil {
			logger.Warn("PC/SC 设备启动期应用卡策略失败",
				"device", worker.ID, "err", err)
		}
	}(w)

	return w, nil
}
