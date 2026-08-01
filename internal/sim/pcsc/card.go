package pcsc

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/ximineko/nyantenna/internal/simaid"
)

// UICC 应用 AID 前缀（TS 101 220）
const (
	AIDPrefixUSIM = "A0000000871002"
	AIDPrefixISIM = "A0000000871004"
)

// 文件标识（TS 31.102 / TS 102 221）
// 显式定型为 uint16：无类型常量直接 byte(...) 会因超出范围而编译失败。
const (
	fidEFICCID  uint16 = 0x2FE2 // MF 下的 ICCID
	fidEFIMSI   uint16 = 0x6F07 // ADF_USIM 下的 IMSI
	fidEFAD     uint16 = 0x6FAD // ADF_USIM 下的管理数据，含 IMSI 里 MNC 的位数
	fidEFSPN    uint16 = 0x6F46 // ADF_USIM 下的服务提供商名称
	fidEFMSISDN uint16 = 0x6F40 // ADF_USIM 下的本机号码
	fidEFSMSP   uint16 = 0x6F42 // ADF_USIM 下的短信参数，含短信中心号码
)

// selectP2NoData 让 SELECT 不返回 FCP 模板。
//
// 返回 FCP 会把 SELECT 变成 61XX → GET RESPONSE 的两段式交互。实测 Alcor AK9563
// 在逻辑通道开关之后会在 GET RESPONSE 上丢失响应同步：本该 0x31 字节的响应被截断
// 成 2 字节，之后整条链路报 SCARD_E_NOT_TRANSACTED，直到重连读卡器才恢复。
//
// 而我们并不需要 FCP——线性定长文件的记录长度可以让卡通过 6C XX 告知。
// 因此所有 SELECT 一律用 P2=0C，把两段式交互彻底消除。
const selectP2NoData = 0x0C

// Card 是一张已连接的 UICC。方法可并发调用，内部串行化——
// 卡本身是单线程设备，同时发两条 APDU 会串包。
type Card struct {
	mu sync.Mutex
	t  Transport
	id string
}

// NewCard 用给定 Transport 构造 Card。id 仅用于日志标识。
func NewCard(t Transport, id string) *Card {
	if strings.TrimSpace(id) == "" {
		id = "pcsc"
	}
	return &Card{t: t, id: id}
}

// ListReaders 枚举系统上的 PC/SC 读卡器名称。
// 未启用 pcsc 构建标签、或 pcscd 未运行时返回错误，调用方按"无读卡器"处理即可。
func ListReaders() ([]string, error) {
	ctx, err := NewContext()
	if err != nil {
		return nil, err
	}
	defer func() { _ = ctx.Close() }()
	return ctx.ListReaders()
}

// Open 连接到读卡器。reader 为空时选择第一个能连上卡的读卡器。
// 调用方负责在用完后 Close。
func Open(reader string) (*Card, error) {
	ctx, err := NewContext()
	if err != nil {
		return nil, err
	}
	readers, err := ctx.ListReaders()
	if err != nil {
		_ = ctx.Close()
		return nil, err
	}
	if len(readers) == 0 {
		_ = ctx.Close()
		return nil, ErrNoReader
	}

	candidates := readers
	if name := strings.TrimSpace(reader); name != "" {
		candidates = nil
		for _, r := range readers {
			if r == name {
				candidates = []string{r}
				break
			}
		}
		if candidates == nil {
			_ = ctx.Close()
			return nil, fmt.Errorf("未找到读卡器 %q（可用: %s）", name, strings.Join(readers, ", "))
		}
	}

	var lastErr error
	for _, r := range candidates {
		t, err := ctx.Connect(r)
		if err != nil {
			lastErr = err
			continue
		}
		return NewCard(&contextBoundTransport{Transport: t, ctx: ctx}, r), nil
	}
	_ = ctx.Close()
	if lastErr == nil {
		lastErr = ErrNoCard
	}
	return nil, lastErr
}

// contextBoundTransport 让 Card.Close 能连带释放资源管理器会话。
type contextBoundTransport struct {
	Transport
	ctx Context
}

func (t *contextBoundTransport) Close() error {
	err := t.Transport.Close()
	if cerr := t.ctx.Close(); err == nil {
		err = cerr
	}
	return err
}

// ID 返回该卡的标识（通常是读卡器名）。
func (c *Card) ID() string { return c.id }

// ATR 返回复位应答，用于诊断电压/协议协商结果。
func (c *Card) ATR() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t.ATR()
}

// Close 断开卡连接。
func (c *Card) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t.Close()
}

// ---------- APDU 底层 ----------

// claForChannel 把逻辑通道号编码进 CLA（ISO 7816-4 §5.1.1）。
// 通道 0-3 占 b2b1；通道 4-19 需置 b7 并用 b4..b1 表示 ch-4。
func claForChannel(cla byte, ch int) (byte, error) {
	switch {
	case ch >= 0 && ch <= 3:
		return (cla &^ 0x43) | byte(ch), nil
	case ch >= 4 && ch <= 19:
		return (cla &^ 0x4F) | 0x40 | byte(ch-4), nil
	default:
		return 0, fmt.Errorf("非法逻辑通道: %d", ch)
	}
}

// withLe 替换或追加 APDU 的 Le 字节，用于 6C XX 重发。
func withLe(apdu []byte, le byte) []byte {
	out := append([]byte(nil), apdu...)
	switch {
	case len(out) == 4: // case 1 → case 2
		return append(out, le)
	case len(out) == 5: // case 2，改 Le
		out[4] = le
		return out
	case len(out) >= 5 && len(out) == 5+int(out[4]): // case 3 → case 4
		return append(out, le)
	case len(out) >= 6 && len(out) == 6+int(out[4]): // case 4，改 Le
		out[len(out)-1] = le
		return out
	default:
		return append(out, le)
	}
}

// exchange 发一条 APDU 并处理两种"再来一次"的状态字：
//   - 6C XX：Le 给错了，卡告诉你正确长度，原样重发
//   - 61 XX：还有 XX 字节没取，发 GET RESPONSE 续读
//
// 返回的 body 不含状态字。
func (c *Card) exchange(apdu []byte) (body []byte, sw1, sw2 byte, err error) {
	if len(apdu) < 4 {
		return nil, 0, 0, fmt.Errorf("APDU 过短: %d", len(apdu))
	}

	resp, err := c.t.Transmit(apdu)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(resp) < 2 {
		return nil, 0, 0, fmt.Errorf("响应过短: %d", len(resp))
	}
	sw1, sw2 = resp[len(resp)-2], resp[len(resp)-1]
	body = resp[:len(resp)-2]

	if sw1 == 0x6C && sw2 != 0x00 {
		resp, err = c.t.Transmit(withLe(apdu, sw2))
		if err != nil {
			return nil, 0, 0, err
		}
		if len(resp) < 2 {
			return nil, 0, 0, fmt.Errorf("6C 重发响应过短: %d", len(resp))
		}
		sw1, sw2 = resp[len(resp)-2], resp[len(resp)-1]
		body = resp[:len(resp)-2]
	}

	// GET RESPONSE 可能连续多轮；限次防止卡片异常导致死循环。
	for i := 0; sw1 == 0x61 && i < 8; i++ {
		get := []byte{apdu[0], 0xC0, 0x00, 0x00, sw2}
		resp, err = c.t.Transmit(get)
		if err != nil {
			return body, sw1, sw2, err
		}
		if len(resp) < 2 {
			return nil, 0, 0, fmt.Errorf("GET RESPONSE 响应过短: %d", len(resp))
		}
		sw1, sw2 = resp[len(resp)-2], resp[len(resp)-1]
		body = append(body, resp[:len(resp)-2]...)
	}
	return body, sw1, sw2, nil
}

func isOK(sw1, sw2 byte) bool { return sw1 == 0x90 && sw2 == 0x00 }

// rewriteSelectP2 把 SELECT 的 P2 从 04（返回 FCP）改写为 0C（不返回数据）。
//
// simaid 的 EF_DIR 读取是给模组路径写的，那边没有本文件顶部描述的同步问题，
// 所以硬编码了 P2=04。这里在 PC/SC 传输层把它改掉；记录长度会由 6C XX 协商补上。
func rewriteSelectP2(apdu []byte) []byte {
	if len(apdu) < 4 || apdu[1] != 0xA4 || apdu[3] != 0x04 {
		return apdu
	}
	out := append([]byte(nil), apdu...)
	out[3] = selectP2NoData
	return out
}

// withTransaction 在事务内执行一段 APDU 序列。调用方必须已持有 c.mu：
// c.mu 只挡住本进程内的并发，事务才挡得住其它 PC/SC 客户端。
func (c *Card) withTransaction(fn func() error) error {
	if err := c.t.BeginTransaction(); err != nil {
		return err
	}
	defer func() { _ = c.t.EndTransaction() }()
	return fn()
}

// fidBytes 拆分文件标识为高低字节。
// 必须经由变量参数转换：byte(常量) 在常量不可表示为 byte 时是编译错误。
func fidBytes(fid uint16) (byte, byte) { return byte(fid >> 8), byte(fid) }

func swError(op string, sw1, sw2 byte) error {
	return fmt.Errorf("%s 失败: SW=%02X%02X", op, sw1, sw2)
}

// ---------- 逻辑通道 ----------

// OpenLogicalChannel 开一个逻辑通道并在其上按 AID 选中应用。
// 选中失败时会把已开的通道关掉，不泄漏通道资源（卡上通常只有 3~4 个）。
func (c *Card) OpenLogicalChannel(aid string) (int, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(aid))
	if err != nil {
		return 0, fmt.Errorf("AID 不是合法 HEX: %w", err)
	}
	if len(raw) == 0 || len(raw) > 16 {
		return 0, fmt.Errorf("AID 长度非法: %d", len(raw))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var ch int
	err = c.withTransaction(func() error {
		// MANAGE CHANNEL — OPEN，由卡分配通道号
		body, sw1, sw2, err := c.exchange([]byte{0x00, 0x70, 0x00, 0x00, 0x01})
		if err != nil {
			return err
		}
		if !isOK(sw1, sw2) {
			return swError("MANAGE CHANNEL(open)", sw1, sw2)
		}
		if len(body) != 1 {
			return fmt.Errorf("MANAGE CHANNEL 未返回通道号: len=%d", len(body))
		}
		ch = int(body[0])

		if err := c.selectAIDLocked(ch, raw); err != nil {
			_ = c.closeLogicalChannelLocked(ch)
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return ch, nil
}

// selectAIDLocked 在指定通道上 SELECT by DF name。
// P2 先用 04（返回 FCP），个别卡只接受 0C（不返回数据），失败后重试一次。
func (c *Card) selectAIDLocked(ch int, aid []byte) error {
	cla, err := claForChannel(0x00, ch)
	if err != nil {
		return err
	}
	var lastSW string
	// 0C 优先：不返回 FCP 就没有 GET RESPONSE，也就没有同步丢失的风险。
	// 个别卡只认 04，因此保留回退。
	for _, p2 := range []byte{selectP2NoData, 0x04} {
		apdu := append([]byte{cla, 0xA4, 0x04, p2, byte(len(aid))}, aid...)
		_, sw1, sw2, err := c.exchange(apdu)
		if err != nil {
			return err
		}
		if isOK(sw1, sw2) {
			return nil
		}
		lastSW = fmt.Sprintf("%02X%02X", sw1, sw2)
	}
	return fmt.Errorf("SELECT AID %X 失败: SW=%s", aid, lastSW)
}

// CloseLogicalChannel 关闭逻辑通道。通道 0 是基础通道，不可关闭。
func (c *Card) CloseLogicalChannel(ch int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.withTransaction(func() error { return c.closeLogicalChannelLocked(ch) })
}

func (c *Card) closeLogicalChannelLocked(ch int) error {
	if ch == 0 {
		return nil
	}
	if ch < 0 || ch > 19 {
		return fmt.Errorf("非法逻辑通道: %d", ch)
	}
	_, sw1, sw2, err := c.exchange([]byte{0x00, 0x70, 0x80, byte(ch)})
	if err != nil {
		return err
	}
	if !isOK(sw1, sw2) {
		return swError("MANAGE CHANNEL(close)", sw1, sw2)
	}
	return nil
}

// closeLogicalChannelLocked 已在 OpenLogicalChannel 的事务内被调用；
// 独立调用时由 CloseLogicalChannel 自行包事务。

// TransmitAPDU 在指定逻辑通道上发送 APDU（HEX 进 HEX 出，含状态字）。
//
// 传入的 APDU 其 CLA 通常是 0x00（调用方把通道号单独传进来），
// 这里负责把通道号编码进 CLA——模组路径下这一步由 AT+CGLA 代劳。
func (c *Card) TransmitAPDU(ch int, hexAPDU string) (string, error) {
	apdu, err := hex.DecodeString(strings.TrimSpace(hexAPDU))
	if err != nil {
		return "", fmt.Errorf("APDU 不是合法 HEX: %w", err)
	}
	if len(apdu) < 4 {
		return "", fmt.Errorf("APDU 过短: %d", len(apdu))
	}
	cla, err := claForChannel(apdu[0], ch)
	if err != nil {
		return "", err
	}
	apdu[0] = cla

	c.mu.Lock()
	defer c.mu.Unlock()

	var out []byte
	if err := c.withTransaction(func() error {
		body, sw1, sw2, err := c.exchange(apdu)
		if err != nil {
			return err
		}
		out = append(append([]byte(nil), body...), sw1, sw2)
		return nil
	}); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(out)), nil
}

// ---------- EF_DIR / AID 解析 ----------

// ListApplications 读 EF_DIR，枚举卡上的应用 AID（USIM/ISIM/CSIM…）。
// 复用 simaid 的实现：它按 FCP 解析记录长度与条数，比盲扫更稳。
func (c *Card) ListApplications() ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var aids [][]byte
	err := c.withTransaction(func() error {
		var terr error
		aids, terr = simaid.ReadDirectoryAIDs(func(apdu []byte) ([]byte, error) {
			body, sw1, sw2, err := c.exchange(rewriteSelectP2(apdu))
			if err != nil {
				return nil, err
			}
			return append(append([]byte(nil), body...), sw1, sw2), nil
		})
		return terr
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(aids))
	for _, aid := range aids {
		out = append(out, strings.ToUpper(hex.EncodeToString(aid)))
	}
	return out, nil
}

// ResolveLogicalChannelAID 实现 sim.LogicalChannelAIDResolver：
// 从 EF_DIR 里找出与前缀匹配的完整 AID。
//
// 这一步不能省：SELECT by DF name 虽然支持部分匹配，但不少卡只认完整 AID，
// 而完整 AID 的尾部（版本号等）各卡不同，只能从卡上读。
func (c *Card) ResolveLogicalChannelAID(app string, fallbackAID string) (string, string, error) {
	prefix := strings.ToUpper(strings.TrimSpace(fallbackAID))
	switch strings.ToLower(strings.TrimSpace(app)) {
	case "usim":
		prefix = AIDPrefixUSIM
	case "isim":
		prefix = AIDPrefixISIM
	}
	if prefix == "" {
		return "", "", fmt.Errorf("无法确定 %q 的 AID 前缀", app)
	}

	apps, err := c.ListApplications()
	if err != nil {
		return "", "", fmt.Errorf("读取 EF_DIR 失败: %w", err)
	}
	for _, aid := range apps {
		if strings.HasPrefix(aid, prefix) && len(aid) > len(prefix) {
			return aid, "ef_dir", nil
		}
	}
	return "", "", fmt.Errorf("EF_DIR 中未找到 %s 应用（前缀 %s，共 %d 个应用）", app, prefix, len(apps))
}
