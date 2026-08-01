package backend

import (
	"fmt"
	"strings"

	"github.com/ximineko/nyantenna/internal/modem"
	"github.com/ximineko/nyantenna/pkg/logger"
)

// 后端模式常量
const (
	BackendAT   = "at"
	BackendQMI  = "qmi"
	BackendMBIM = "mbim"
	// BackendPCSC 是"只有 PC/SC 读卡器、没有基带"的设备：
	// 仅提供 SIM 文件读取与 AKA，专供 VoWiFi 使用。
	BackendPCSC = "pcsc"
)

// NormalizeBackendMode 标准化后端模式字符串
func NormalizeBackendMode(in string) string {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "", BackendAT:
		return BackendAT // 默认 AT 模式
	case BackendQMI:
		return BackendQMI
	case BackendMBIM:
		return BackendMBIM
	case BackendPCSC:
		return BackendPCSC
	default:
		return BackendAT
	}
}

// ValidateBackendMode 验证后端模式是否有效
func ValidateBackendMode(in string) error {
	switch NormalizeBackendMode(in) {
	case BackendAT, BackendQMI, BackendMBIM, BackendPCSC:
		return nil
	default:
		return fmt.Errorf("无效的 device_backend 值: %q (可选: at, qmi, mbim, pcsc)", in)
	}
}

// NewBackend 根据配置模式创建对应后端实例的工厂方法
// mode: "at" | "qmi"
// controlPath: QMI 控制设备路径（qmi 模式必须）
// m: modem.Manager（at 模式必须）
// source: QMI Core 资源源（qmi 模式必须）
func NewBackend(mode, controlPath string, m *modem.Manager, source QMISource, mbimSource MBIMSource) (DeviceBackend, error) {
	mode = NormalizeBackendMode(mode)

	switch mode {
	case BackendAT:
		if m == nil {
			return nil, fmt.Errorf("AT 模式需要 modem.Manager")
		}
		logger.Info("[backend] 使用 AT 后端模式")
		return NewATBackend(m), nil

	case BackendQMI:
		b, err := NewQMIBackend(controlPath, source)
		if err != nil {
			return nil, fmt.Errorf("QMI 后端初始化失败: %w", err)
		}
		logger.Info("[backend] 使用 QMI 后端模式", "control_path", controlPath)
		return b, nil

	case BackendMBIM:
		if mbimSource == nil {
			return nil, fmt.Errorf("MBIM 模式需要 MBIMSource")
		}
		logger.Info("[backend] 使用 MBIM 后端模式", "control_path", controlPath)
		return NewMBIMBackend(controlPath, mbimSource), nil

	case BackendPCSC:
		// PC/SC 后端需要一个已连上的读卡器句柄，无法从这里的参数构造；
		// 由设备层拿到 Card 之后直接 NewPCSCBackend。
		return nil, fmt.Errorf("PC/SC 后端不经由 NewBackend 构造，请使用 NewPCSCBackend")

	default:
		return nil, fmt.Errorf("不支持的后端模式: %s", mode)
	}
}
