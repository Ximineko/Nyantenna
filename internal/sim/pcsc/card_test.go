package pcsc

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// fakeTransport 按"请求 HEX → 响应 HEX"脚本作答，用于在无硬件条件下验证 APDU 逻辑。
type fakeTransport struct {
	script   map[string]string
	sent     []string
	txDepth  int
	txBegins int
	// sentOutsideTx 记录未被事务包住就发出的 APDU——这类序列会被其它
	// PC/SC 客户端插入打断，是 SCARD_E_NOT_TRANSACTED 的根源
	sentOutsideTx []string

	// beginErr / transmitErr 注入一次性故障：命中一次后自动清除，
	// 用于验证句柄失效 → 重连 → 重试的自愈路径。
	beginErr     error
	transmitErr  map[string]error
	reconnects   int
	reconnectErr error
}

func newFake(script map[string]string) *fakeTransport {
	up := make(map[string]string, len(script))
	for k, v := range script {
		up[strings.ToUpper(k)] = strings.ToUpper(v)
	}
	return &fakeTransport{script: up}
}

func (f *fakeTransport) Transmit(apdu []byte) ([]byte, error) {
	key := strings.ToUpper(hex.EncodeToString(apdu))
	f.sent = append(f.sent, key)
	if f.txDepth <= 0 {
		f.sentOutsideTx = append(f.sentOutsideTx, key)
	}
	if err, ok := f.transmitErr[key]; ok {
		delete(f.transmitErr, key)
		return nil, err
	}
	resp, ok := f.script[key]
	if !ok {
		// 未编排的指令一律回 6A82（文件未找到），模拟卡的拒绝行为
		return []byte{0x6A, 0x82}, nil
	}
	return hex.DecodeString(resp)
}

func (f *fakeTransport) ATR() []byte  { return []byte{0x3B, 0x9F, 0x00, 0x01} }
func (f *fakeTransport) Close() error { return nil }

// 事务在假实现里只做计数，用于断言"多条 APDU 的序列确实被包住了"
func (f *fakeTransport) BeginTransaction() error {
	if err := f.beginErr; err != nil {
		f.beginErr = nil
		return err
	}
	f.txDepth++
	f.txBegins++
	return nil
}
func (f *fakeTransport) EndTransaction() error { f.txDepth--; return nil }

func (f *fakeTransport) Reconnect() error { f.reconnects++; return f.reconnectErr }

func TestClaForChannel(t *testing.T) {
	tests := []struct {
		cla  byte
		ch   int
		want byte
		bad  bool
	}{
		{0x00, 0, 0x00, false},
		{0x00, 1, 0x01, false},
		{0x00, 3, 0x03, false},
		// 通道 4 起改用"扩展"编码：置 b7，低四位存 ch-4
		{0x00, 4, 0x40, false},
		{0x00, 19, 0x4F, false},
		// 已带通道位的 CLA 要被覆盖而不是叠加
		{0x03, 1, 0x01, false},
		{0x00, 20, 0x00, true},
		{0x00, -1, 0x00, true},
	}
	for _, tt := range tests {
		got, err := claForChannel(tt.cla, tt.ch)
		if tt.bad {
			if err == nil {
				t.Fatalf("claForChannel(%#x,%d) 应报错", tt.cla, tt.ch)
			}
			continue
		}
		if err != nil {
			t.Fatalf("claForChannel(%#x,%d) 意外报错: %v", tt.cla, tt.ch, err)
		}
		if got != tt.want {
			t.Fatalf("claForChannel(%#x,%d) = %#x, want %#x", tt.cla, tt.ch, got, tt.want)
		}
	}
}

func TestWithLe(t *testing.T) {
	tests := []struct {
		name string
		in   string
		le   byte
		want string
	}{
		{"case1 追加 Le", "00A40004", 0x10, "00A4000410"},
		{"case2 替换 Le", "00B0000000", 0x09, "00B0000009"},
		{"case3 追加 Le", "00A4040407A0000000871002", 0x20, "00A4040407A000000087100220"},
		{"case4 替换 Le", "00A4040407A000000087100200", 0x20, "00A4040407A000000087100220"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, _ := hex.DecodeString(tt.in)
			got := strings.ToUpper(hex.EncodeToString(withLe(in, tt.le)))
			if got != tt.want {
				t.Fatalf("withLe(%s, %#x) = %s, want %s", tt.in, tt.le, got, tt.want)
			}
		})
	}
}

func TestDecodeIMSI(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		bad  bool
	}{
		// 15 位（奇数）：首半字节 9 = 奇数位指示，其后为 IMSI
		{"15 位", "08 49 06 10 10 32 54 76 98", "460010123456789", false},
		// 14 位（偶数）：首半字节 8，末尾补 F
		{"14 位补 F", "08 48 06 10 10 32 54 76 F8", "46001012345678", false},
		{"长度字段越界", "08 09", "", true},
		{"空数据", "", "", true},
		{"含非数字", "08 09 A0 10 32 54 76 98 10", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := hex.DecodeString(strings.ReplaceAll(tt.raw, " ", ""))
			if err != nil {
				t.Fatalf("测试数据非法: %v", err)
			}
			got, err := decodeIMSI(raw)
			if tt.bad {
				if err == nil {
					t.Fatalf("decodeIMSI(%s) 应报错，却得到 %q", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeIMSI(%s) 报错: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("decodeIMSI(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// EF_DIR 里的 AID 通常比前缀长（尾部带版本号），解析必须返回完整 AID，
// 否则挑食的卡会拒绝 SELECT。
func TestResolveLogicalChannelAIDFromEFDIR(t *testing.T) {
	const usimFull = "A0000000871002FF49FF0589"
	const isimFull = "A0000000871004FF49FF0589"

	card := NewCard(newFake(map[string]string{
		"00A4000C023F00": "9000",
		"00A4000C022F00": "620E8205422100220283022F008A0105" + "9000",
		"00B2010422":     "61124F0C" + usimFull + "500255539000",
		"00B2020422":     "61124F0C" + isimFull + "500249539000",
		"00B2030422":     "6A83",
	}), "test")

	aid, source, err := card.ResolveLogicalChannelAID("usim", AIDPrefixUSIM)
	if err != nil {
		t.Fatalf("解析 USIM AID 失败: %v", err)
	}
	if aid != usimFull {
		t.Fatalf("USIM AID = %q, want %q", aid, usimFull)
	}
	if source != "ef_dir" {
		t.Fatalf("source = %q", source)
	}

	aid, _, err = card.ResolveLogicalChannelAID("isim", AIDPrefixISIM)
	if err != nil {
		t.Fatalf("解析 ISIM AID 失败: %v", err)
	}
	if aid != isimFull {
		t.Fatalf("ISIM AID = %q, want %q", aid, isimFull)
	}
}

func TestResolveLogicalChannelAIDMissingISIM(t *testing.T) {
	card := NewCard(newFake(map[string]string{
		"00A4000C023F00": "9000",
		"00A4000C022F00": "620E8205422100220283022F008A0105" + "9000",
		"00B2010422":     "610A4F07A00000008710029000",
		"00B2020422":     "6A83",
	}), "test")

	if _, _, err := card.ResolveLogicalChannelAID("isim", AIDPrefixISIM); err == nil {
		t.Fatal("卡上没有 ISIM 时应报错，好让上层回退 USIM")
	}
}

// 开通道 → SELECT AID；SELECT 失败必须把通道关掉，否则卡上有限的通道会被耗尽。
func TestOpenLogicalChannelClosesOnSelectFailure(t *testing.T) {
	f := newFake(map[string]string{
		"0070000001": "019000", // 分配通道 1
		"00708001":   "9000",   // 关闭通道
		// SELECT 两种 P2 都不认
		"01A4040C07A0000000871002": "6A82",
		"01A4040407A0000000871002": "6A82",
	})
	card := NewCard(f, "test")

	if _, err := card.OpenLogicalChannel(AIDPrefixUSIM); err == nil {
		t.Fatal("SELECT 失败时 OpenLogicalChannel 应报错")
	}
	closed := false
	for _, s := range f.sent {
		if strings.HasPrefix(s, "007080") {
			closed = true
		}
	}
	if !closed {
		t.Fatalf("SELECT 失败后应关闭逻辑通道，实际发出: %v", f.sent)
	}
}

// 默认用 P2=0C（不返回 FCP，规避 GET RESPONSE 同步问题）；
// 个别卡只认 04，此时要能回退。
func TestOpenLogicalChannelFallsBackToP2_04(t *testing.T) {
	f := newFake(map[string]string{
		"0070000001":               "029000",
		"02A4040C07A0000000871002": "6A86", // 首选的 P2=0C 不被接受
		"02A4040407A0000000871002": "9000", // 回退到 P2=04
	})
	card := NewCard(f, "test")

	ch, err := card.OpenLogicalChannel(AIDPrefixUSIM)
	if err != nil {
		t.Fatalf("应回退到 P2=04 并成功: %v", err)
	}
	if ch != 2 {
		t.Fatalf("通道号 = %d, want 2", ch)
	}
	if f.sent[1] != "02A4040C07A0000000871002" {
		t.Fatalf("应先尝试 P2=0C，实际首个 SELECT 是 %s", f.sent[1])
	}
}

// TransmitAPDU 收到的 APDU CLA 是 0x00（通道号单独传入），
// 必须由本层写进 CLA——模组路径下这件事由 AT+CGLA 代劳。
func TestTransmitAPDUAppliesChannelToCLA(t *testing.T) {
	authAPDU := "008800812210" + strings.Repeat("AB", 16) + "10" + strings.Repeat("CD", 16)
	onChannel1 := "01" + authAPDU[2:]

	f := newFake(map[string]string{
		onChannel1: "DB08" + strings.Repeat("11", 8) + "10" + strings.Repeat("22", 16) + "10" + strings.Repeat("33", 16) + "9000",
	})
	card := NewCard(f, "test")

	resp, err := card.TransmitAPDU(1, authAPDU)
	if err != nil {
		t.Fatalf("TransmitAPDU 失败: %v", err)
	}
	if !strings.HasPrefix(resp, "DB08") || !strings.HasSuffix(resp, "9000") {
		t.Fatalf("响应异常: %s", resp)
	}
	if f.sent[0] != onChannel1 {
		t.Fatalf("CLA 未写入通道号: 发出 %s, want %s", f.sent[0], onChannel1)
	}
}

// T=0 的卡常以 61 XX 告知"还有数据"，需要 GET RESPONSE 续读，
// 且 GET RESPONSE 的 CLA 必须沿用同一通道。
func TestExchangeHandlesGetResponse(t *testing.T) {
	f := newFake(map[string]string{
		"01B0000009": "6109",
		"01C0000009": "080910103254769810" + "9000",
	})
	card := NewCard(f, "test")

	resp, err := card.TransmitAPDU(1, "00B0000009")
	if err != nil {
		t.Fatalf("TransmitAPDU 失败: %v", err)
	}
	if resp != "0809101032547698109000" {
		t.Fatalf("续读结果 = %s", resp)
	}
}

// 6C XX 表示 Le 给错了，卡直接告知正确长度，需原样重发。
func TestExchangeHandlesWrongLe(t *testing.T) {
	f := newFake(map[string]string{
		"00B0000000": "6C09",
		"00B0000009": "0809101032547698109000",
	})
	card := NewCard(f, "test")

	resp, err := card.TransmitAPDU(0, "00B0000000")
	if err != nil {
		t.Fatalf("TransmitAPDU 失败: %v", err)
	}
	if resp != "0809101032547698109000" {
		t.Fatalf("重发结果 = %s", resp)
	}
}

func TestReadIMSI(t *testing.T) {
	const usimFull = "A0000000871002FF49FF0589"
	f := newFake(map[string]string{
		"00A4000C023F00":        "9000",
		"00A4000C022F00":        "620E8205422100220283022F008A01059000",
		"00B2010422":            "61124F0C" + usimFull + "500255539000",
		"00B2020422":            "6A83",
		"0070000001":            "019000",
		"01A4040C0C" + usimFull: "9000",
		"01A4000C026F07":        "9000",
		"01B0000009":            "0849061010325476989000",
		"00708001":              "9000",
	})
	card := NewCard(f, "test")

	imsi, err := card.ReadIMSI()
	if err != nil {
		t.Fatalf("ReadIMSI 失败: %v", err)
	}
	if imsi != "460010123456789" {
		t.Fatalf("IMSI = %q", imsi)
	}
}

// 句柄失效（USB 自动挂起、pcscd 侧复位等）后 BeginTransaction 就会报错，
// 此时必须重连并重试，否则只能重启进程才能恢复。
func TestWithTransactionReconnectsOnStaleBegin(t *testing.T) {
	f := newFake(map[string]string{
		"00B0000009": "0809101032547698109000",
	})
	f.beginErr = errStaleHandleForTest
	card := NewCard(f, "test")

	resp, err := card.TransmitAPDU(0, "00B0000009")
	if err != nil {
		t.Fatalf("句柄失效后应重连重试成功: %v", err)
	}
	if resp != "0809101032547698109000" {
		t.Fatalf("重试结果 = %s", resp)
	}
	if f.reconnects != 1 {
		t.Fatalf("应重连 1 次，实际 %d 次", f.reconnects)
	}
}

// 句柄失效也可能在事务中途的 Transmit 上暴露，同样要重连后原样重试整个序列。
func TestWithTransactionReconnectsOnStaleTransmit(t *testing.T) {
	f := newFake(map[string]string{
		"00B0000009": "0809101032547698109000",
	})
	f.transmitErr = map[string]error{"00B0000009": errStaleHandleForTest}
	card := NewCard(f, "test")

	resp, err := card.TransmitAPDU(0, "00B0000009")
	if err != nil {
		t.Fatalf("Transmit 句柄失效后应重连重试成功: %v", err)
	}
	if resp != "0809101032547698109000" {
		t.Fatalf("重试结果 = %s", resp)
	}
	if f.reconnects != 1 {
		t.Fatalf("应重连 1 次，实际 %d 次", f.reconnects)
	}
	if f.txBegins != 2 {
		t.Fatalf("重试应重新取事务，txBegins = %d, want 2", f.txBegins)
	}
}

// 与句柄无关的错误（如卡拒绝、协议错误）不该触发重连，避免把真实故障掩盖成重试风暴。
func TestWithTransactionNoReconnectOnOtherErrors(t *testing.T) {
	f := newFake(nil)
	f.beginErr = errors.New("boom")
	card := NewCard(f, "test")

	if _, err := card.TransmitAPDU(0, "00B0000009"); err == nil {
		t.Fatal("非句柄类错误应原样上抛")
	}
	if f.reconnects != 0 {
		t.Fatalf("非句柄类错误不应重连，实际重连 %d 次", f.reconnects)
	}
}

// 重连本身失败（读卡器被拔走等）时，错误里应同时带上原始错误与重连错误。
func TestWithTransactionReconnectFailure(t *testing.T) {
	f := newFake(nil)
	f.beginErr = errStaleHandleForTest
	f.reconnectErr = errors.New("reader gone")
	card := NewCard(f, "test")

	_, err := card.TransmitAPDU(0, "00B0000009")
	if err == nil {
		t.Fatal("重连失败时应报错")
	}
	if !errors.Is(err, errStaleHandleForTest) {
		t.Fatalf("错误应保留原始句柄失效错误: %v", err)
	}
	if !strings.Contains(err.Error(), "reader gone") {
		t.Fatalf("错误应包含重连失败原因: %v", err)
	}
}

// 未启用 pcsc 构建标签时，入口应给出可操作的提示而不是 panic。
// 带标签构建时会走真实实现，其成败取决于 pcscd 与硬件，不在本用例断言范围内。
func TestNewContextWithoutBuildTag(t *testing.T) {
	ctx, err := NewContext()
	if err == nil {
		_ = ctx.Close()
		t.Skip("本次以 -tags pcsc 构建且 pcscd 可用")
	}
	if !errors.Is(err, ErrPCSCUnavailable) {
		t.Skipf("本次以 -tags pcsc 构建，NewContext 走真实实现: %v", err)
	}
	if !strings.Contains(err.Error(), "pcsc") {
		t.Fatalf("错误信息应提示如何启用: %v", err)
	}
}
