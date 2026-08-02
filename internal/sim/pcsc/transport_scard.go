//go:build pcsc

package pcsc

import (
	"errors"
	"fmt"

	"github.com/ebfe/scard"
)

// scardContext 是 PC/SC 资源管理器会话的真实实现（cgo → libpcsclite）。
type scardContext struct {
	ctx *scard.Context
}

func newContext() (Context, error) {
	ctx, err := scard.EstablishContext()
	if err != nil {
		return nil, fmt.Errorf("建立 PC/SC 会话失败: %w", err)
	}
	return &scardContext{ctx: ctx}, nil
}

func (c *scardContext) ListReaders() ([]string, error) {
	readers, err := c.ctx.ListReaders()
	if err != nil {
		return nil, fmt.Errorf("枚举读卡器失败: %w", err)
	}
	return readers, nil
}

func (c *scardContext) Connect(reader string) (Transport, error) {
	// 共享模式即可：我们只发 APDU，不需要独占重置卡片。
	// T=0/T=1 交给驱动协商——UICC 两种都可能。
	card, err := c.ctx.Connect(reader, scard.ShareShared, scard.ProtocolT0|scard.ProtocolT1)
	if err != nil {
		return nil, fmt.Errorf("连接读卡器 %q 失败: %w", reader, err)
	}
	status, err := card.Status()
	if err != nil {
		_ = card.Disconnect(scard.LeaveCard)
		return nil, fmt.Errorf("读取卡状态失败: %w", err)
	}
	return &scardTransport{card: card, atr: append([]byte(nil), status.Atr...)}, nil
}

func (c *scardContext) Close() error { return c.ctx.Release() }

type scardTransport struct {
	card *scard.Card
	atr  []byte
}

func (t *scardTransport) Transmit(apdu []byte) ([]byte, error) {
	resp, err := t.card.Transmit(apdu)
	if err != nil {
		return nil, fmt.Errorf("APDU 传输失败: %w", err)
	}
	return resp, nil
}

func (t *scardTransport) BeginTransaction() error {
	if err := t.card.BeginTransaction(); err != nil {
		return fmt.Errorf("取得卡事务失败: %w", err)
	}
	return nil
}

// EndTransaction 用 LeaveCard 结束：复位会让已打开的逻辑通道与文件选择全部失效。
func (t *scardTransport) EndTransaction() error {
	return t.card.EndTransaction(scard.LeaveCard)
}

// Reconnect 用 SCardReconnect 换取新句柄。LeaveCard：不复位卡片，
// 已打开的逻辑通道与文件选择保持有效，上层可以原样重试失败的序列。
func (t *scardTransport) Reconnect() error {
	if err := t.card.Reconnect(scard.ShareShared, scard.ProtocolT0|scard.ProtocolT1, scard.LeaveCard); err != nil {
		return fmt.Errorf("重连读卡器失败: %w", err)
	}
	if status, err := t.card.Status(); err == nil {
		t.atr = append([]byte(nil), status.Atr...)
	}
	return nil
}

// isScardStaleHandle 识别"不重连不会恢复"的 scard 错误码。
// 实测 ak9563 在跑一段时间后固定报 SCARD_E_INVALID_VALUE；
// NOT_TRANSACTED 是 GET RESPONSE 同步丢失后的表现（见 card.go 顶部注释），同样只有重连能救；
// RESET_CARD 按 PC/SC 规范本就要求以 SCardReconnect 应答。
func isScardStaleHandle(err error) bool {
	var se scard.Error
	if !errors.As(err, &se) {
		return false
	}
	switch se {
	case scard.ErrInvalidValue, scard.ErrInvalidHandle, scard.ErrNotTransacted,
		scard.ErrResetCard, scard.ErrRemovedCard, scard.ErrReaderUnavailable:
		return true
	}
	return false
}

func (t *scardTransport) ATR() []byte { return t.atr }

// Close 使用 LeaveCard：不复位卡片，避免打断可能并存的其它会话。
func (t *scardTransport) Close() error { return t.card.Disconnect(scard.LeaveCard) }
