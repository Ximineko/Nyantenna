package pcsc

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

// 各 EF 的读取长度（TS 31.102）
const (
	efIMSILen   = 9  // 1 字节长度指示 + 8 字节 BCD
	efICCIDLen  = 10 // 定长 10 字节 BCD
	efADLen     = 4  // 只需前 4 字节，第 4 字节低半字节是 MNC 位数
	efSPNLen    = 17 // 1 字节显示条件 + 16 字节名称
	msisdnTail  = 14 // MSISDN 记录尾部定长部分：长度+TON/NPI+10 字节号码+CCP+EXT
	efMSISDNMax = 40 // 记录长度上限，足够覆盖常见 Alpha Identifier
	smspTail    = 28 // SMSP 记录尾部定长部分：参数指示 1 + 目的地址 12 + 服务中心地址 12 + PID/DCS/VP 3
	efSMSPMax   = 52 // 记录长度上限
)

// USIMSession 是一条已选中 ADF_USIM 的逻辑通道。
//
// 逐个文件开关通道会让卡上有限的逻辑通道反复申请释放，也拖慢启动；
// 用一条会话把要读的 EF 一次读完更合适。用完必须 Close。
type USIMSession struct {
	c   *Card
	ch  int
	cla byte
}

// OpenUSIM 解析 USIM AID、开通道并选中它。
func (c *Card) OpenUSIM() (*USIMSession, error) {
	aid, _, err := c.ResolveLogicalChannelAID("usim", AIDPrefixUSIM)
	if err != nil {
		return nil, err
	}
	ch, err := c.OpenLogicalChannel(aid)
	if err != nil {
		return nil, fmt.Errorf("打开 USIM 逻辑通道失败: %w", err)
	}
	cla, err := claForChannel(0x00, ch)
	if err != nil {
		_ = c.CloseLogicalChannel(ch)
		return nil, err
	}
	return &USIMSession{c: c, ch: ch, cla: cla}, nil
}

// Close 关闭该会话占用的逻辑通道。
func (s *USIMSession) Close() error {
	if s == nil || s.c == nil {
		return nil
	}
	return s.c.CloseLogicalChannel(s.ch)
}

// selectEF 在会话通道上选中一个 EF。
func (s *USIMSession) selectEF(fid uint16) error {
	hi, lo := fidBytes(fid)
	_, sw1, sw2, err := s.c.exchange([]byte{s.cla, 0xA4, 0x00, selectP2NoData, 0x02, hi, lo})
	if err != nil {
		return err
	}
	if !isOK(sw1, sw2) {
		return swError(fmt.Sprintf("SELECT %04X", fid), sw1, sw2)
	}
	return nil
}

// readBinary 读透明文件。length 为 0 时由卡通过 6C XX 告知实际长度。
func (s *USIMSession) readBinary(fid uint16, length int) ([]byte, error) {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()

	var out []byte
	if err := s.c.withTransaction(func() error {
		if err := s.selectEF(fid); err != nil {
			return err
		}
		body, sw1, sw2, err := s.c.exchange([]byte{s.cla, 0xB0, 0x00, 0x00, byte(length)})
		if err != nil {
			return err
		}
		if !isOK(sw1, sw2) {
			return swError(fmt.Sprintf("READ BINARY %04X", fid), sw1, sw2)
		}
		out = body
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// readRecord 读线性定长文件的某条记录。
func (s *USIMSession) readRecord(fid uint16, record, length int) ([]byte, error) {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()

	var out []byte
	if err := s.c.withTransaction(func() error {
		if err := s.selectEF(fid); err != nil {
			return err
		}
		body, sw1, sw2, err := s.c.exchange([]byte{s.cla, 0xB2, byte(record), 0x04, byte(length)})
		if err != nil {
			return err
		}
		if !isOK(sw1, sw2) {
			return swError(fmt.Sprintf("READ RECORD %04X#%d", fid, record), sw1, sw2)
		}
		out = body
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// IMSI 读取 EF_IMSI。
func (s *USIMSession) IMSI() (string, error) {
	raw, err := s.readBinary(fidEFIMSI, efIMSILen)
	if err != nil {
		return "", err
	}
	return decodeIMSI(raw)
}

// MNCLength 读取 EF_AD 第 4 字节低半字节，即 IMSI 中 MNC 的位数（2 或 3）。
// 拿不到或取值异常时回落到 2——绝大多数网络是 2 位。
func (s *USIMSession) MNCLength() int {
	raw, err := s.readBinary(fidEFAD, efADLen)
	if err != nil || len(raw) < 4 {
		return 2
	}
	if n := int(raw[3] & 0x0F); n == 2 || n == 3 {
		return n
	}
	return 2
}

// SPN 读取 EF_SPN 服务提供商名称。文件不存在时返回空串而非错误——
// 不是所有卡都写了 SPN。
func (s *USIMSession) SPN() string {
	raw, err := s.readBinary(fidEFSPN, efSPNLen)
	if err != nil || len(raw) < 2 {
		return ""
	}
	return decodeAlphaField(raw[1:])
}

// MSISDN 读取 EF_MSISDN 第一条非空记录里的本机号码。
// 很多卡根本没写这个文件，返回空串不算错误。
func (s *USIMSession) MSISDN() string {
	for rec := 1; rec <= 4; rec++ {
		raw, err := s.readRecord(fidEFMSISDN, rec, efMSISDNMax)
		if err != nil {
			// 记录不存在或文件缺失，不再往下试
			return ""
		}
		if number := decodeMSISDNRecord(raw); number != "" {
			return number
		}
	}
	return ""
}

// SMSC 读取 EF_SMSP 里的短信中心号码。
// MO 短信要用它；读不到返回空串，上层会以空 SMSC 继续启动。
func (s *USIMSession) SMSC() string {
	for rec := 1; rec <= 4; rec++ {
		raw, err := s.readRecord(fidEFSMSP, rec, efSMSPMax)
		if err != nil {
			return ""
		}
		if number := decodeSMSPRecord(raw); number != "" {
			return number
		}
	}
	return ""
}

// ReadICCID 读取 EF_ICCID。该文件在 MF 下，不需要选 ADF，
// 因此直接走基础通道，不必开逻辑通道。
func (c *Card) ReadICCID() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hi, lo := fidBytes(fidEFICCID)
	var raw []byte
	if err := c.withTransaction(func() error {
		if _, sw1, sw2, err := c.exchange([]byte{0x00, 0xA4, 0x00, selectP2NoData, 0x02, hi, lo}); err != nil {
			return err
		} else if !isOK(sw1, sw2) {
			return swError("SELECT EF_ICCID", sw1, sw2)
		}
		body, sw1, sw2, err := c.exchange([]byte{0x00, 0xB0, 0x00, 0x00, efICCIDLen})
		if err != nil {
			return err
		}
		if !isOK(sw1, sw2) {
			return swError("READ BINARY EF_ICCID", sw1, sw2)
		}
		raw = body
		return nil
	}); err != nil {
		return "", err
	}
	return decodeICCID(raw)
}

// ReadIMSI 是只读 IMSI 的便捷封装，内部自建一次性 USIM 会话。
func (c *Card) ReadIMSI() (string, error) {
	s, err := c.OpenUSIM()
	if err != nil {
		return "", err
	}
	defer func() { _ = s.Close() }()
	return s.IMSI()
}

// ---------- 编解码 ----------

// swapNibbles 把 BCD 半字节交换后拼成字符串（低半字节在前）。
func swapNibbles(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b) * 2)
	for _, v := range b {
		sb.WriteByte(nibbleChar(v & 0x0F))
		sb.WriteByte(nibbleChar(v >> 4))
	}
	return sb.String()
}

func nibbleChar(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + (n - 10)
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// decodeIMSI 解码 EF_IMSI（TS 31.102 §4.2.2）。
//
// 布局：首字节是后续有效字节数，其后为半字节交换的 BCD。
// 交换后的第一个半字节是奇偶/类型指示位，不属于 IMSI 本身；
// 位数为偶时末尾用 F 补齐。
func decodeIMSI(raw []byte) (string, error) {
	if len(raw) < 2 {
		return "", fmt.Errorf("EF_IMSI 数据过短: %d", len(raw))
	}
	n := int(raw[0])
	if n <= 0 || 1+n > len(raw) {
		return "", fmt.Errorf("EF_IMSI 长度字段非法: %d（可用 %d）", n, len(raw)-1)
	}
	s := swapNibbles(raw[1 : 1+n])
	if len(s) < 2 {
		return "", fmt.Errorf("EF_IMSI 解码结果过短")
	}
	s = strings.TrimRight(s[1:], "fF")
	if len(s) < 5 || len(s) > 15 {
		return "", fmt.Errorf("IMSI 长度异常: %d", len(s))
	}
	if !allDigits(s) {
		return "", fmt.Errorf("IMSI 含非数字字符: %q", s)
	}
	return s, nil
}

// decodeICCID 解码 EF_ICCID（TS 102 221 §13.2）：定长 10 字节，
// 半字节交换，无长度前缀，位数为奇时末尾补 F。
func decodeICCID(raw []byte) (string, error) {
	if len(raw) < 5 {
		return "", fmt.Errorf("EF_ICCID 数据过短: %d", len(raw))
	}
	s := strings.TrimRight(swapNibbles(raw), "fF")
	if len(s) < 10 || len(s) > 20 {
		return "", fmt.Errorf("ICCID 长度异常: %d", len(s))
	}
	if !allDigits(s) {
		return "", fmt.Errorf("ICCID 含非数字字符: %q", s)
	}
	return s, nil
}

// decodeMSISDNRecord 解析 EF_MSISDN 记录（TS 31.102 §4.2.26）。
//
// 记录尾部 14 字节是定长的：
//
//	[BCD 长度 1][TON/NPI 1][号码 10][CCP 1][EXT 1]
//
// 前面变长的部分是 Alpha Identifier。TON=国际号码时补 "+"。
func decodeMSISDNRecord(rec []byte) string {
	if len(rec) < msisdnTail {
		return ""
	}
	tail := rec[len(rec)-msisdnTail:]
	bcdLen := int(tail[0])
	// bcdLen 含 TON/NPI 那一字节；0xFF 表示空记录
	if bcdLen < 2 || bcdLen > 11 {
		return ""
	}
	ton := tail[1]
	digits := strings.TrimRight(swapNibbles(tail[2:2+bcdLen-1]), "fF")
	if !allDigits(digits) {
		return ""
	}
	// TON/NPI 的 bit7..4 = 001 表示国际号码
	if ton&0x70 == 0x10 {
		return "+" + digits
	}
	return digits
}

// decodeSMSPRecord 从 EF_SMSP 记录里取出短信中心号码（TS 31.102 §4.2.27）。
//
// 记录尾部 28 字节定长：
//
//	[参数指示 1][TP-目的地址 12][TS-服务中心地址 12][PID 1][DCS 1][VP 1]
//
// 服务中心地址本身是 [长度 1][TON/NPI 1][号码 10]，其中长度含 TON/NPI 那一字节。
func decodeSMSPRecord(rec []byte) string {
	if len(rec) < smspTail {
		return ""
	}
	tail := rec[len(rec)-smspTail:]
	// 参数指示位为 0 表示对应参数存在；bit2(0x02) 对应服务中心地址
	if tail[0] != 0xFF && tail[0]&0x02 != 0 {
		return ""
	}
	sc := tail[13:25]
	bcdLen := int(sc[0])
	if bcdLen < 2 || bcdLen > 11 {
		return ""
	}
	ton := sc[1]
	digits := strings.TrimRight(swapNibbles(sc[2:2+bcdLen-1]), "fF")
	if !allDigits(digits) {
		return ""
	}
	if ton&0x70 == 0x10 {
		return "+" + digits
	}
	return digits
}

// decodeAlphaField 解码 SIM 的 Alpha 字段：首字节 0x80/0x81/0x82 表示 UCS2，
// 否则按 GSM 03.38 默认字母表处理——对 ASCII 范围内的名称与直接按字节取值等价，
// 而运营商名几乎都落在这个范围内。
func decodeAlphaField(raw []byte) string {
	raw = trimFF(raw)
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == 0x80 {
		return decodeUCS2BE(raw[1:])
	}
	// 0x81/0x82 是带基址的压缩 UCS2，较为罕见；这里不展开，只做保守回退
	if raw[0] == 0x81 || raw[0] == 0x82 {
		return ""
	}
	var sb strings.Builder
	for _, b := range raw {
		if b == 0x00 {
			break
		}
		if b < 0x20 || b > 0x7E {
			continue
		}
		sb.WriteByte(b)
	}
	return strings.TrimSpace(sb.String())
}

func decodeUCS2BE(raw []byte) string {
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		u := uint16(raw[i])<<8 | uint16(raw[i+1])
		if u == 0 || u == 0xFFFF {
			break
		}
		units = append(units, u)
	}
	return strings.TrimSpace(string(utf16.Decode(units)))
}

func trimFF(raw []byte) []byte {
	end := len(raw)
	for end > 0 && raw[end-1] == 0xFF {
		end--
	}
	return raw[:end]
}
