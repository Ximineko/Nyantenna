package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ximineko/nyantenna/pkg/logger"
)

// ErrPCSCNoRadio 表示该能力依赖基带，而 PC/SC 读卡器只有卡、没有射频。
//
// 这类方法不该被静默成功：谎报"已注册/信号良好"会让上层健康检查与
// 状态展示得出完全错误的结论。调用方应当按后端模式跳过，而不是吞掉错误。
var ErrPCSCNoRadio = errors.New("PC/SC 读卡器没有基带，该能力不可用")

// PCSCCard 是 PC/SC 后端所需的读卡器能力。
//
// 定义在本包而不是直接引用 internal/sim/pcsc，是为了避开导入环：
// internal/sim 依赖 internal/backend，而 pcsc 依赖 internal/sim。
type PCSCCard interface {
	ID() string
	ReadICCID() (string, error)
	OpenUSIMFiles() (PCSCUSIMFiles, error)

	OpenLogicalChannel(aid string) (int, error)
	CloseLogicalChannel(channel int) error
	TransmitAPDU(channel int, hexAPDU string) (string, error)
	ResolveLogicalChannelAID(app string, fallbackAID string) (string, string, error)

	Close() error
}

// PCSCUSIMFiles 是一条已选中 ADF_USIM 的会话，用完必须 Close。
type PCSCUSIMFiles interface {
	IMSI() (string, error)
	MNCLength() int
	SPN() string
	MSISDN() string
	SMSC() string
	Close() error
}

// pcscIdentity 缓存一次性读出的卡内信息。
// 这些值对同一张卡是固定的，而上层会高频轮询设备状态，
// 每次都去走 APDU 既慢又会和 AKA 抢卡。
type pcscIdentity struct {
	IMSI      string
	ICCID     string
	MSISDN    string
	SPN       string
	SMSC      string
	MNCLength int
	loaded    bool
}

// PCSCBackend 是仅有读卡器、没有基带的设备后端。
//
// SIM 相关能力（AKA、IMSI/ICCID/SPN 等文件）为真实实现；
// 射频相关能力（信号、驻网、短信、开关机模式）一律返回 ErrPCSCNoRadio。
// IMEI 无法从卡上读取，由配置提供——它只用于 IKEv2 IDi 与 IMS 头部。
type PCSCBackend struct {
	card PCSCCard
	imei string

	mu       sync.Mutex
	identity pcscIdentity
}

// 编译期确认满足完整的后端契约。
var (
	_ DeviceBackend      = (*PCSCBackend)(nil)
	_ SIMAuthAIDResolver = (*PCSCBackend)(nil)
	_ SMSCProvider       = (*PCSCBackend)(nil)
)

// NewPCSCBackend 构造 PC/SC 后端。imei 为配置中手工填写的值。
func NewPCSCBackend(card PCSCCard, imei string) *PCSCBackend {
	return &PCSCBackend{card: card, imei: strings.TrimSpace(imei)}
}

// Mode 返回后端模式标识。
func (b *PCSCBackend) Mode() string { return BackendPCSC }

// Close 断开读卡器连接。
func (b *PCSCBackend) Close() error {
	if b == nil || b.card == nil {
		return nil
	}
	return b.card.Close()
}

// loadIdentity 用一条 USIM 会话把要用的文件一次读完并缓存。
func (b *PCSCBackend) loadIdentity() (pcscIdentity, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.identity.loaded {
		return b.identity, nil
	}
	if b.card == nil {
		return pcscIdentity{}, errors.New("PC/SC 读卡器未初始化")
	}

	id := pcscIdentity{MNCLength: 2}

	// ICCID 在 MF 下，与 ADF 无关，单独读
	if iccid, err := b.card.ReadICCID(); err != nil {
		logger.Warn("[pcsc] 读取 ICCID 失败", "reader", b.card.ID(), "err", err)
	} else {
		id.ICCID = iccid
	}

	files, err := b.card.OpenUSIMFiles()
	if err != nil {
		return pcscIdentity{}, fmt.Errorf("打开 USIM 会话失败: %w", err)
	}
	defer func() { _ = files.Close() }()

	imsi, err := files.IMSI()
	if err != nil {
		return pcscIdentity{}, fmt.Errorf("读取 IMSI 失败: %w", err)
	}
	id.IMSI = imsi
	id.MNCLength = files.MNCLength()
	id.SPN = files.SPN()
	id.MSISDN = files.MSISDN()
	id.SMSC = files.SMSC()
	id.loaded = true

	b.identity = id
	logger.Info("[pcsc] 卡内身份已读取",
		"reader", b.card.ID(),
		"imsi_len", len(id.IMSI),
		"iccid_len", len(id.ICCID),
		"mnc_len", id.MNCLength,
		"spn", id.SPN,
		"has_msisdn", id.MSISDN != "",
		"has_smsc", id.SMSC != "")
	return id, nil
}

// InvalidateIdentity 丢弃缓存，下次读取重新走卡。换卡后调用。
func (b *PCSCBackend) InvalidateIdentity() {
	b.mu.Lock()
	b.identity = pcscIdentity{}
	b.mu.Unlock()
}

// ---------- DeviceInfoProvider ----------

// GetIMEI 返回配置中填写的 IMEI。读卡器没有基带，卡上也不存 IMEI。
func (b *PCSCBackend) GetIMEI(context.Context) (string, error) {
	if b.imei == "" {
		return "", errors.New("PC/SC 设备未配置 IMEI，请在设备配置中手工填写")
	}
	return b.imei, nil
}

func (b *PCSCBackend) GetIMSI(context.Context) (string, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return "", err
	}
	return id.IMSI, nil
}

func (b *PCSCBackend) GetICCID(context.Context) (string, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return "", err
	}
	return id.ICCID, nil
}

func (b *PCSCBackend) GetMSISDN(context.Context) (string, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return "", err
	}
	return id.MSISDN, nil
}

// GetRevision 指的是模组固件版本，读卡器场景下不存在。
func (b *PCSCBackend) GetRevision(context.Context) (string, error) {
	return "", ErrPCSCNoRadio
}

func (b *PCSCBackend) GetSignalInfo(context.Context) (*SignalInfo, error) {
	return nil, ErrPCSCNoRadio
}

func (b *PCSCBackend) GetServingSystem(context.Context) (*ServingSystem, error) {
	return nil, ErrPCSCNoRadio
}

// IsSimInserted 以能否读出 IMSI 为准——卡不在读卡器里就读不到。
func (b *PCSCBackend) IsSimInserted(context.Context) (bool, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return false, err
	}
	return id.IMSI != "", nil
}

// GetNativeMCCMNC 按 EF_AD 给出的 MNC 位数切分 IMSI。
func (b *PCSCBackend) GetNativeMCCMNC(context.Context) (string, string, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return "", "", err
	}
	if len(id.IMSI) < 3+id.MNCLength {
		return "", "", fmt.Errorf("IMSI 过短，无法切分 MCC/MNC: %d 位", len(id.IMSI))
	}
	return id.IMSI[:3], id.IMSI[3 : 3+id.MNCLength], nil
}

func (b *PCSCBackend) GetNativeSPN(context.Context) (string, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return "", err
	}
	return id.SPN, nil
}

// GetSIMMetadata 目前只提供归属 PLMN 与 SPN 相关的部分。
// PNN/OPL/服务表需要再读若干 EF，暂未实现——缺失时上层会回落到 SPN 或纯 PLMN 展示。
func (b *PCSCBackend) GetSIMMetadata(context.Context) (*SIMMetadata, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return nil, err
	}
	meta := &SIMMetadata{}
	if len(id.IMSI) >= 3+id.MNCLength {
		meta.NativeMCC = id.IMSI[:3]
		meta.NativeMNC = id.IMSI[3 : 3+id.MNCLength]
	}
	return meta, nil
}

// GetSMSC 返回卡上 EF_SMSP 里的短信中心号码。MO 短信经 IMS 发出时要用。
func (b *PCSCBackend) GetSMSC(context.Context) (string, error) {
	id, err := b.loadIdentity()
	if err != nil {
		return "", err
	}
	return id.SMSC, nil
}

// ---------- 实时身份读取 ----------
//
// 设备层用 liveSIMIdentityReader 判断"这张卡还是不是原来那张"。
// 缓存对同一张卡有效，但换卡后必须重读，因此 Live 变体先作废缓存。

func (b *PCSCBackend) GetIMSILive(ctx context.Context) (string, error) {
	b.InvalidateIdentity()
	return b.GetIMSI(ctx)
}

func (b *PCSCBackend) GetICCIDLive(ctx context.Context) (string, error) {
	// GetIMSILive 已经作废过缓存时不必再作废一次，但这里无从得知调用顺序，
	// 而重复作废只是多读一轮文件，代价可接受。
	b.InvalidateIdentity()
	return b.GetICCID(ctx)
}

func (b *PCSCBackend) GetNativeSPNLive(ctx context.Context) (string, error) {
	return b.GetNativeSPN(ctx)
}

func (b *PCSCBackend) GetSIMMetadataLive(ctx context.Context) (*SIMMetadata, error) {
	return b.GetSIMMetadata(ctx)
}

// ---------- SMSProvider ----------
//
// 读卡器收发不了短信。VoWiFi 场景下 MT/MO 短信走 IMS，
// 由 vowifi 侧的 SIP MESSAGE 通道处理，不经过本后端。

func (b *PCSCBackend) SendSMS(context.Context, string, string) error { return ErrPCSCNoRadio }
func (b *PCSCBackend) ReadSMS(context.Context, int) (*SMS, error)    { return nil, ErrPCSCNoRadio }
func (b *PCSCBackend) DeleteSMS(context.Context, int) error          { return ErrPCSCNoRadio }
func (b *PCSCBackend) ListSMS(context.Context) ([]SMSSummary, error) { return nil, ErrPCSCNoRadio }
func (b *PCSCBackend) DeleteAllSMS(context.Context) error            { return ErrPCSCNoRadio }

// ---------- OperatingModeController ----------

// SetOperatingMode 空实现：没有射频可开关。
// 返回 nil 而不是错误，是因为调用方的语义是"确保射频关闭"，
// 而这对读卡器恒为真；返回错误只会在 VoWiFi 启动路径上刷无意义的告警。
func (b *PCSCBackend) SetOperatingMode(_ context.Context, mode OperatingMode) error {
	logger.Debug("[pcsc] 忽略射频模式切换（读卡器无基带）", "mode", int(mode))
	return nil
}

// GetOperatingMode 恒定返回 ModeRFOff：没有射频，等价于永久飞行模式。
// 这样 VoWiFi 启动路径会认定"已处于飞行模式"，跳过冗余切换。
func (b *PCSCBackend) GetOperatingMode(context.Context) (OperatingMode, error) {
	return ModeRFOff, nil
}

// Reboot 无从谈起：没有模组可重启。
func (b *PCSCBackend) Reboot(context.Context) error { return ErrPCSCNoRadio }

// ---------- SIMAuthProvider ----------
//
// 这部分才是 PC/SC 后端存在的意义：逻辑通道与 APDU 直通到卡。

func (b *PCSCBackend) OpenLogicalChannel(_ context.Context, aid string) (int, error) {
	if b.card == nil {
		return 0, errors.New("PC/SC 读卡器未初始化")
	}
	return b.card.OpenLogicalChannel(aid)
}

func (b *PCSCBackend) CloseLogicalChannel(_ context.Context, channelID int) error {
	if b.card == nil {
		return errors.New("PC/SC 读卡器未初始化")
	}
	return b.card.CloseLogicalChannel(channelID)
}

func (b *PCSCBackend) TransmitAPDU(_ context.Context, channelID int, command string) (string, error) {
	if b.card == nil {
		return "", errors.New("PC/SC 读卡器未初始化")
	}
	return b.card.TransmitAPDU(channelID, command)
}

// ResolveSIMAuthAID 从 EF_DIR 解析完整 AID（不少卡拒绝按前缀 SELECT）。
func (b *PCSCBackend) ResolveSIMAuthAID(_ context.Context, app string, fallbackAID string) (string, string, error) {
	if b.card == nil {
		return "", "", errors.New("PC/SC 读卡器未初始化")
	}
	return b.card.ResolveLogicalChannelAID(app, fallbackAID)
}
