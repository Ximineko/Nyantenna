package pcsc

import (
	"errors"
	"strings"
	"time"

	swusim "github.com/iniwex5/vowifi-go/engine/sim"
	"github.com/ximineko/nyantenna/internal/sim"
	"github.com/ximineko/nyantenna/pkg/logger"
)

// ErrNotAModem 表示该操作只有真实模组才能提供（读卡器没有基带）。
var ErrNotAModem = errors.New("PC/SC 读卡器不支持 AT 命令")

// Modem 把 Card 适配成 sim.ATModem，从而复用既有的 ATAKAProvider。
//
// ATAKAProvider 在 AKA 路径上只用到 DeviceID / OpenLogicalChannel /
// CloseLogicalChannel / TransmitAPDU 四个方法，ExecuteATSilent 不在其中；
// 这里保留该方法只为满足接口，调用即报错。
//
// 同时实现 sim.LogicalChannelAIDResolver（由 Card 提供，走 EF_DIR）——
// 没有它的话 resolveLogicalChannelAID 会直接返回 sim_auth_aid_not_ready。
type Modem struct {
	*Card
}

// 编译期确认两个接口都满足。
var (
	_ sim.ATModem                   = (*Modem)(nil)
	_ sim.LogicalChannelAIDResolver = (*Modem)(nil)
)

// NewModem 把一张已连接的卡包装成 ATModem。
func NewModem(c *Card) *Modem { return &Modem{Card: c} }

// DeviceID 返回卡标识，用于日志串联。
func (m *Modem) DeviceID() string { return m.Card.ID() }

// ExecuteATSilent 恒定失败：读卡器没有基带，不存在 AT 通道。
func (m *Modem) ExecuteATSilent(string, time.Duration) (string, error) {
	return "", ErrNotAModem
}

// NewAKAProvider 用 PC/SC 读卡器构造 VoWiFi 所需的 AKAProvider。
//
// preference 取值同 sim 包：usim / auto / isim / isim_strict。
// 相比模组路径，这里的 ISIM 逻辑通道走标准 MANAGE CHANNEL，
// 成功率通常更高，因此把 auto 作为推荐值。
func NewAKAProvider(c *Card, preference string) (swusim.AKAProvider, error) {
	if c == nil {
		return nil, errors.New("card 为空")
	}
	p := sim.NewATAKAProvider(NewModem(c))
	if preference == "" {
		preference = sim.AKAAppPreferenceAuto
	}
	return sim.WrapPreferredAKAProvider(p, preference), nil
}

// OpenAKAProvider 是最常用的一步到位入口：连读卡器、读 IMSI、返回 AKAProvider。
//
// reader 为空时自动选第一个能连上卡的读卡器。
// 返回的 Card 需由调用方在停止使用后 Close。
func OpenAKAProvider(reader, preference string) (*Card, swusim.AKAProvider, string, error) {
	card, err := Open(reader)
	if err != nil {
		return nil, nil, "", err
	}

	apps, err := card.ListApplications()
	if err != nil {
		_ = card.Close()
		return nil, nil, "", err
	}
	hasISIM := false
	for _, aid := range apps {
		if strings.HasPrefix(aid, AIDPrefixISIM) {
			hasISIM = true
		}
	}
	logger.Info("PC/SC 卡片就绪",
		"reader", card.ID(),
		"atr", maskBytes(card.ATR()),
		"apps", len(apps),
		"isim", hasISIM)

	imsi, err := card.ReadIMSI()
	if err != nil {
		_ = card.Close()
		return nil, nil, "", err
	}

	provider, err := NewAKAProvider(card, preference)
	if err != nil {
		_ = card.Close()
		return nil, nil, "", err
	}
	return card, provider, imsi, nil
}

// maskBytes 只保留首尾各 2 字节，避免把卡片指纹整段写进日志。
func maskBytes(b []byte) string {
	const hexDigits = "0123456789abcdef"
	if len(b) == 0 {
		return ""
	}
	enc := func(src []byte) string {
		out := make([]byte, 0, len(src)*2)
		for _, c := range src {
			out = append(out, hexDigits[c>>4], hexDigits[c&0x0F])
		}
		return string(out)
	}
	if len(b) <= 4 {
		return enc(b)
	}
	return enc(b[:2]) + "..." + enc(b[len(b)-2:])
}
