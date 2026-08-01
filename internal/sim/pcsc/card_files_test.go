package pcsc

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestDecodeICCID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		bad  bool
	}{
		// 19 位（奇数）：末尾补 F
		{"19 位补 F", "984405214365870921F3", "8944501234567890123", false},
		// 20 位（偶数）：无补位
		{"20 位", "98440521436587092134", "89445012345678901243", false},
		{"过短", "9844", "", true},
		{"含非数字", "98AB05214365870921F3", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := hex.DecodeString(tt.raw)
			if err != nil {
				t.Fatalf("测试数据非法: %v", err)
			}
			got, err := decodeICCID(raw)
			if tt.bad {
				if err == nil {
					t.Fatalf("decodeICCID(%s) 应报错，却得到 %q", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeICCID(%s) 报错: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("decodeICCID(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDecodeMSISDNRecord(t *testing.T) {
	tests := []struct {
		name string
		rec  string
		want string
	}{
		// TON/NPI=0x91 是国际号码，要补 "+"
		{"国际号码", "FFFF0891683108108300F0FFFFFFFFFF", "+8613800138000"},
		// TON/NPI=0x81 是本地号码，不补
		{"本地号码", "FFFF0881683108108300F0FFFFFFFFFF", "8613800138000"},
		// 空记录：长度字节为 FF
		{"空记录", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF", ""},
		{"记录过短", "FFFF08", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := hex.DecodeString(tt.rec)
			if err != nil {
				t.Fatalf("测试数据非法: %v", err)
			}
			if got := decodeMSISDNRecord(rec); got != tt.want {
				t.Fatalf("decodeMSISDNRecord(%s) = %q, want %q", tt.rec, got, tt.want)
			}
		})
	}
}

func TestDecodeAlphaField(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"ASCII", "43544578636C" + strings.Repeat("FF", 4), "CTExcl"},
		{"全 FF 视为空", strings.Repeat("FF", 8), ""},
		// 首字节 0x80 表示后续为大端 UCS2
		{"UCS2", "80" + "4E2D" + "56FD" + "FFFF", "中国"},
		{"空输入", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := hex.DecodeString(tt.raw)
			if err != nil {
				t.Fatalf("测试数据非法: %v", err)
			}
			if got := decodeAlphaField(raw); got != tt.want {
				t.Fatalf("decodeAlphaField(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// EF_AD 第 4 字节低半字节是 IMSI 里 MNC 的位数；读不到或取值异常时回落 2 位。
func TestUSIMSessionMNCLength(t *testing.T) {
	const usimFull = "A0000000871002FF49FF0589"
	base := map[string]string{
		"00A4000C023F00":        "9000",
		"00A4000C022F00":        "620E8205422100220283022F008A01059000",
		"00B2010422":            "61124F0C" + usimFull + "500255539000",
		"00B2020422":            "6A83",
		"0070000001":            "019000",
		"01A4040C0C" + usimFull: "9000",
		"00708001":              "9000",
		"01A4000C026FAD":        "9000",
	}

	cases := []struct {
		name string
		ad   string
		want int
	}{
		{"3 位 MNC", "000000039000", 3},
		{"2 位 MNC", "000000029000", 2},
		{"取值异常回落 2", "0000000F9000", 2},
		{"读取失败回落 2", "6A82", 2},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			script := make(map[string]string, len(base)+1)
			for k, v := range base {
				script[k] = v
			}
			script["01B0000004"] = tt.ad

			card := NewCard(newFake(script), "test")
			s, err := card.OpenUSIM()
			if err != nil {
				t.Fatalf("OpenUSIM 失败: %v", err)
			}
			defer func() { _ = s.Close() }()

			if got := s.MNCLength(); got != tt.want {
				t.Fatalf("MNCLength() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ICCID 在 MF 下，不需要开逻辑通道——走基础通道即可。
func TestReadICCIDUsesBasicChannel(t *testing.T) {
	f := newFake(map[string]string{
		"00A4000C022FE2": "9000",
		"00B000000A":     "984405214365870921F39000",
	})
	card := NewCard(f, "test")

	iccid, err := card.ReadICCID()
	if err != nil {
		t.Fatalf("ReadICCID 失败: %v", err)
	}
	if iccid != "8944501234567890123" {
		t.Fatalf("ICCID = %q", iccid)
	}
	for _, sent := range f.sent {
		if strings.HasPrefix(sent, "0070") {
			t.Fatalf("读 ICCID 不应开逻辑通道，实际发出: %v", f.sent)
		}
	}
}

// SELECT 与随后的 READ 之间若被其它 PC/SC 客户端插入，卡上的当前文件选择就没了，
// 表现为 SCARD_E_NOT_TRANSACTED。因此所有多条 APDU 的序列都必须包在事务里。
func TestAllAPDUsRunInsideTransaction(t *testing.T) {
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
		"01A4000C026FAD":        "9000",
		"01B0000004":            "000000029000",
		"00708001":              "9000",
		"00A4000C022FE2":        "9000",
		"00B000000A":            "984405214365870921F39000",
		"008800812210AB":        "6A82",
	})
	card := NewCard(f, "test")

	if _, err := card.ReadICCID(); err != nil {
		t.Fatalf("ReadICCID 失败: %v", err)
	}
	s, err := card.OpenUSIM()
	if err != nil {
		t.Fatalf("OpenUSIM 失败: %v", err)
	}
	if _, err := s.IMSI(); err != nil {
		t.Fatalf("IMSI 失败: %v", err)
	}
	_ = s.MNCLength()
	if err := s.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	if _, err := card.TransmitAPDU(1, "008800812210AB"); err != nil {
		t.Fatalf("TransmitAPDU 失败: %v", err)
	}

	if len(f.sentOutsideTx) > 0 {
		t.Fatalf("有 %d 条 APDU 未被事务包住: %v", len(f.sentOutsideTx), f.sentOutsideTx)
	}
	if f.txBegins == 0 {
		t.Fatal("未开启过任何事务")
	}
	if f.txDepth != 0 {
		t.Fatalf("事务未正确闭合，残留深度 %d", f.txDepth)
	}
	t.Logf("共 %d 条 APDU，分 %d 个事务", len(f.sent), f.txBegins)
}

// 回归：SELECT 一律用 P2=0C。用 04 会让卡返回 FCP，从而触发
// 61XX → GET RESPONSE 两段式交互——实测某些读卡器会在此丢失响应同步，
// 表现为响应被截断成 2 字节、随后整条链路 SCARD_E_NOT_TRANSACTED。
func TestSelectNeverRequestsFCP(t *testing.T) {
	const usimFull = "A0000000871002FF49FF0589"
	f := newFake(map[string]string{
		"00A4000C023F00":        "9000",
		"00A4000C022F00":        "9000",
		"00B2010400":            "6C32",
		"00B2010432":            "61124F0C" + usimFull + "500255539000",
		"00B2020400":            "6A83",
		"0070000001":            "019000",
		"01A4040C0C" + usimFull: "9000",
		"01A4000C026F07":        "9000",
		"01B0000009":            "0849061010325476989000",
		"00708001":              "9000",
		"00A4000C022FE2":        "9000",
		"00B000000A":            "984405214365870921F39000",
	})
	card := NewCard(f, "test")

	if _, err := card.ReadICCID(); err != nil {
		t.Fatalf("ReadICCID 失败: %v", err)
	}
	if _, err := card.ReadIMSI(); err != nil {
		t.Fatalf("ReadIMSI 失败: %v", err)
	}

	for _, sent := range f.sent {
		// SELECT 的 P2 是第 4 个字节，即 HEX 的第 7-8 位
		if len(sent) >= 8 && sent[2:4] == "A4" && sent[6:8] == "04" {
			t.Fatalf("出现了会返回 FCP 的 SELECT: %s（全部: %v）", sent, f.sent)
		}
	}
	// 记录长度应由 6C XX 协商得到，而不是从 FCP 解析
	var sawLenNegotiation bool
	for _, sent := range f.sent {
		if sent == "00B2010432" {
			sawLenNegotiation = true
		}
	}
	if !sawLenNegotiation {
		t.Fatalf("未见 6C XX 协商出的记录长度重发，实际: %v", f.sent)
	}
}
