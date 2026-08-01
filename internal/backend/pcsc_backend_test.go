package backend

import (
	"context"
	"errors"
	"testing"
)

type fakeUSIMFiles struct {
	imsi    string
	imsiErr error
	mncLen  int
	spn     string
	msisdn  string
	smsc    string
	closed  bool
}

func (f *fakeUSIMFiles) IMSI() (string, error) { return f.imsi, f.imsiErr }
func (f *fakeUSIMFiles) MNCLength() int        { return f.mncLen }
func (f *fakeUSIMFiles) SPN() string           { return f.spn }
func (f *fakeUSIMFiles) MSISDN() string        { return f.msisdn }
func (f *fakeUSIMFiles) SMSC() string          { return f.smsc }
func (f *fakeUSIMFiles) Close() error          { f.closed = true; return nil }

type fakeCard struct {
	iccid     string
	iccidErr  error
	files     *fakeUSIMFiles
	openErr   error
	openCount int
}

var _ PCSCCard = (*fakeCard)(nil)

func (c *fakeCard) ID() string                             { return "fake-reader" }
func (c *fakeCard) ReadICCID() (string, error)             { return c.iccid, c.iccidErr }
func (c *fakeCard) Close() error                           { return nil }
func (c *fakeCard) OpenLogicalChannel(string) (int, error) { return 1, nil }
func (c *fakeCard) CloseLogicalChannel(int) error          { return nil }
func (c *fakeCard) TransmitAPDU(int, string) (string, error) {
	return "9000", nil
}
func (c *fakeCard) ResolveLogicalChannelAID(string, string) (string, string, error) {
	return "A0000000871002FF", "ef_dir", nil
}
func (c *fakeCard) OpenUSIMFiles() (PCSCUSIMFiles, error) {
	c.openCount++
	if c.openErr != nil {
		return nil, c.openErr
	}
	return c.files, nil
}

func newTestBackend() (*PCSCBackend, *fakeCard) {
	card := &fakeCard{
		iccid: "8944501234567890123",
		files: &fakeUSIMFiles{imsi: "234331234567890", mncLen: 2, spn: "CTExcel", msisdn: "+447700900123", smsc: "+447785016005"},
	}
	return NewPCSCBackend(card, "351756051523999"), card
}

func TestPCSCBackendReadsCardIdentity(t *testing.T) {
	b, _ := newTestBackend()
	ctx := context.Background()

	if v, err := b.GetIMSI(ctx); err != nil || v != "234331234567890" {
		t.Fatalf("GetIMSI = %q, %v", v, err)
	}
	if v, err := b.GetICCID(ctx); err != nil || v != "8944501234567890123" {
		t.Fatalf("GetICCID = %q, %v", v, err)
	}
	if v, err := b.GetMSISDN(ctx); err != nil || v != "+447700900123" {
		t.Fatalf("GetMSISDN = %q, %v", v, err)
	}
	if v, err := b.GetNativeSPN(ctx); err != nil || v != "CTExcel" {
		t.Fatalf("GetNativeSPN = %q, %v", v, err)
	}
	if v, err := b.GetSMSC(ctx); err != nil || v != "+447785016005" {
		t.Fatalf("GetSMSC = %q, %v", v, err)
	}
	// IMEI 只能来自配置
	if v, err := b.GetIMEI(ctx); err != nil || v != "351756051523999" {
		t.Fatalf("GetIMEI = %q, %v", v, err)
	}
}

// EF_AD 给出的 MNC 位数决定 MCC/MNC 的切分点，不能写死 2 位。
func TestPCSCBackendNativeMCCMNCUsesEFADLength(t *testing.T) {
	for _, tt := range []struct {
		name    string
		imsi    string
		mncLen  int
		wantMCC string
		wantMNC string
	}{
		{"2 位 MNC", "234331234567890", 2, "234", "33"},
		{"3 位 MNC", "310260123456789", 3, "310", "260"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			card := &fakeCard{files: &fakeUSIMFiles{imsi: tt.imsi, mncLen: tt.mncLen}}
			b := NewPCSCBackend(card, "1")
			mcc, mnc, err := b.GetNativeMCCMNC(context.Background())
			if err != nil {
				t.Fatalf("GetNativeMCCMNC 报错: %v", err)
			}
			if mcc != tt.wantMCC || mnc != tt.wantMNC {
				t.Fatalf("= %q/%q, want %q/%q", mcc, mnc, tt.wantMCC, tt.wantMNC)
			}
		})
	}
}

// 卡内信息对同一张卡固定不变，上层又高频轮询，必须只读一次。
func TestPCSCBackendCachesIdentity(t *testing.T) {
	b, card := newTestBackend()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := b.GetIMSI(ctx); err != nil {
			t.Fatalf("GetIMSI 报错: %v", err)
		}
	}
	if card.openCount != 1 {
		t.Fatalf("USIM 会话打开了 %d 次，应只有 1 次", card.openCount)
	}

	// Live 变体用于判断是否换卡，必须强制重读
	if _, err := b.GetIMSILive(ctx); err != nil {
		t.Fatalf("GetIMSILive 报错: %v", err)
	}
	if card.openCount != 2 {
		t.Fatalf("GetIMSILive 后打开次数 = %d，应为 2", card.openCount)
	}
}

// 射频类能力必须明确报错，不能静默返回零值——
// 否则上层会把"没有基带"误读成"信号为 0 / 未注册"。
func TestPCSCBackendRadioCapabilitiesFail(t *testing.T) {
	b, _ := newTestBackend()
	ctx := context.Background()

	if _, err := b.GetSignalInfo(ctx); !errors.Is(err, ErrPCSCNoRadio) {
		t.Fatalf("GetSignalInfo 应返回 ErrPCSCNoRadio，实际 %v", err)
	}
	if _, err := b.GetServingSystem(ctx); !errors.Is(err, ErrPCSCNoRadio) {
		t.Fatalf("GetServingSystem 应返回 ErrPCSCNoRadio，实际 %v", err)
	}
	if err := b.SendSMS(ctx, "1", "x"); !errors.Is(err, ErrPCSCNoRadio) {
		t.Fatalf("SendSMS 应返回 ErrPCSCNoRadio，实际 %v", err)
	}
	if err := b.Reboot(ctx); !errors.Is(err, ErrPCSCNoRadio) {
		t.Fatalf("Reboot 应返回 ErrPCSCNoRadio，实际 %v", err)
	}
}

// VoWiFi 启动路径靠 GetOperatingMode 判断"是否已在飞行模式"来跳过冗余切换。
// 读卡器没有射频，等价于永久飞行，必须报 ModeRFOff 且 SetOperatingMode 不报错。
func TestPCSCBackendReportsPermanentFlightMode(t *testing.T) {
	b, _ := newTestBackend()
	ctx := context.Background()

	mode, err := b.GetOperatingMode(ctx)
	if err != nil {
		t.Fatalf("GetOperatingMode 报错: %v", err)
	}
	if mode != ModeRFOff {
		t.Fatalf("GetOperatingMode = %d, want ModeRFOff(%d)", mode, ModeRFOff)
	}
	if err := b.SetOperatingMode(ctx, ModeRFOff); err != nil {
		t.Fatalf("SetOperatingMode 不应报错: %v", err)
	}
}

func TestPCSCBackendRequiresConfiguredIMEI(t *testing.T) {
	card := &fakeCard{files: &fakeUSIMFiles{imsi: "234331234567890", mncLen: 2}}
	b := NewPCSCBackend(card, "  ")
	if _, err := b.GetIMEI(context.Background()); err == nil {
		t.Fatal("未配置 IMEI 时应报错，好让用户知道要手工填写")
	}
}

func TestPCSCBackendModeAndValidation(t *testing.T) {
	b, _ := newTestBackend()
	if b.Mode() != BackendPCSC {
		t.Fatalf("Mode() = %q, want %q", b.Mode(), BackendPCSC)
	}
	if got := NormalizeBackendMode("PCSC"); got != BackendPCSC {
		t.Fatalf("NormalizeBackendMode(\"PCSC\") = %q", got)
	}
	if err := ValidateBackendMode("pcsc"); err != nil {
		t.Fatalf("ValidateBackendMode(\"pcsc\") 报错: %v", err)
	}
}
