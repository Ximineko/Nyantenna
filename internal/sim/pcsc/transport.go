// Package pcsc 通过 PC/SC 读卡器直接访问 UICC，提供与模组等价的 AKA 能力。
//
// VoWiFi 链路对 SIM 的唯一要求是 AKA 计算（CalculateAKA），而它落到卡上
// 就是一条 ISO 7816-4 / TS 31.102 的 AUTHENTICATE APDU。卡不关心这条 APDU
// 来自模组还是读卡器，因此把读卡器包装成 sim.ATModem 之后，既有的
// ATAKAProvider（USIM/ISIM 偏好、带 Le 变体重试、GET RESPONSE 续读）可以原样复用。
//
// 与模组路径相比的取舍：
//   - 逻辑通道走标准 MANAGE CHANNEL，不再受各家 AT+CCHO/AT+CGLA 实现差异的折磨；
//     ISIM 通常比模组路径更容易拿到。
//   - 卡从模组里取出后不再有蜂窝驻网能力，且焊死的 eUICC(MFF2) 无法使用本路径。
//   - 读卡器需支持 SIM 的电压等级（Class B 3V / Class C 1.8V），仅 5V 的老读卡器不可用。
//
// 真正的 PC/SC 绑定依赖 cgo(libpcsclite)，被隔离在 `pcsc` 构建标签之后，
// 默认的 CGO_ENABLED=0 构建不受影响。本文件及 card.go/modem.go 均为纯 Go，
// 始终参与编译，可用假 Transport 做单元测试。
package pcsc

import "errors"

// ErrPCSCUnavailable 表示当前二进制未启用 PC/SC 支持。
// 启用方式：CGO_ENABLED=1 go build -tags pcsc ...
var ErrPCSCUnavailable = errors.New("当前构建未启用 PC/SC 支持（需 CGO_ENABLED=1 且 -tags pcsc）")

// ErrNoReader 表示没有找到任何读卡器。
var ErrNoReader = errors.New("未找到 PC/SC 读卡器")

// ErrNoCard 表示读卡器里没有卡（或卡未就绪）。
var ErrNoCard = errors.New("读卡器中没有可用的卡")

// Transport 是一次已连接的卡会话，只负责裸 APDU 收发。
// 实现方不做任何状态字处理，61XX/6CXX 由上层 Card 统一处理。
type Transport interface {
	// Transmit 发送一条完整 APDU，返回响应（含末尾 2 字节状态字）。
	Transmit(apdu []byte) ([]byte, error)
	// BeginTransaction 取得卡的独占访问。
	//
	// PC/SC 是多客户端的：SELECT 与随后的 READ 之间若被其它客户端（或 pcscd 的
	// 存在性轮询）插入，卡上的当前文件选择就没了，表现为 SCARD_E_NOT_TRANSACTED
	// 之类的传输层错误。凡是多条 APDU 才有意义的序列都必须包在事务里。
	BeginTransaction() error
	// EndTransaction 释放独占访问，且不复位卡片。
	EndTransaction() error
	// ATR 返回复位应答，仅用于诊断（判断卡片类型/电压协商结果）。
	ATR() []byte
	// Close 断开与卡的连接。
	Close() error
}

// Context 是一次 PC/SC 资源管理器会话。
type Context interface {
	// ListReaders 列出当前系统上的读卡器名称。
	ListReaders() ([]string, error)
	// Connect 连接到指定读卡器中的卡。
	Connect(reader string) (Transport, error)
	// Close 释放资源管理器会话。
	Close() error
}

// NewContext 建立 PC/SC 会话。
// 未启用 pcsc 构建标签时固定返回 ErrPCSCUnavailable。
func NewContext() (Context, error) { return newContext() }
