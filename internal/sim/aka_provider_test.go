package sim

import (
	"errors"
	"reflect"
	"testing"
	"time"

	swusim "github.com/iniwex5/vowifi-go/engine/sim"
)

var _ swusim.AKAProvider = (*ATAKAProvider)(nil)

type akaWithPreferenceProviderStub struct {
	result             swusim.AKAResult
	err                error
	lastPreferenceUsed string
}

func (s *akaWithPreferenceProviderStub) CalculateAKAWithPreference(rand16, autn16 []byte, preference string) (swusim.AKAResult, error) {
	s.lastPreferenceUsed = preference
	return s.result, s.err
}

type akaProviderModemFake struct {
	basicErr         error
	logicalCalls     []string
	logicalAIDs      []string
	logicalClosed    []int
	logicalResponses []string
	openLogicalErr   error
	openErrByAID     map[string]error
	openErrOnceByAID map[string]error
	openChannelByAID map[string]int
	executeCalls     []string
	resolvedAID      string
	resolvedAIDByApp map[string]string
	resolveSource    string
	resolveErr       error
}

func (f *akaProviderModemFake) DeviceID() string { return "dev-at" }

func (f *akaProviderModemFake) ExecuteATSilent(cmd string, timeout time.Duration) (string, error) {
	f.executeCalls = append(f.executeCalls, cmd)
	return "", errors.New("ExecuteATSilent should not be used for SIMAuth APDU")
}

func (f *akaProviderModemFake) OpenLogicalChannel(aid string) (int, error) {
	f.logicalAIDs = append(f.logicalAIDs, aid)
	if f.openErrOnceByAID != nil {
		if err := f.openErrOnceByAID[aid]; err != nil {
			delete(f.openErrOnceByAID, aid)
			return 0, err
		}
	}
	if f.openErrByAID != nil {
		if err := f.openErrByAID[aid]; err != nil {
			return 0, err
		}
	}
	if f.openLogicalErr != nil {
		return 0, f.openLogicalErr
	}
	if f.openChannelByAID != nil {
		if ch := f.openChannelByAID[aid]; ch != 0 {
			return ch, nil
		}
	}
	return 1, nil
}

func (f *akaProviderModemFake) ResolveLogicalChannelAID(app string, fallbackAID string) (string, string, error) {
	if f.resolveErr != nil {
		return "", "", f.resolveErr
	}
	if f.resolvedAIDByApp != nil {
		if aid := f.resolvedAIDByApp[app]; aid != "" {
			return aid, f.resolveSource, nil
		}
	}
	if f.resolvedAID == "" {
		return fallbackAID, "fallback_test", nil
	}
	return f.resolvedAID, f.resolveSource, nil
}
func (f *akaProviderModemFake) CloseLogicalChannel(channel int) error {
	f.logicalClosed = append(f.logicalClosed, channel)
	return nil
}
func (f *akaProviderModemFake) TransmitAPDU(channel int, hexAPDU string) (string, error) {
	f.logicalCalls = append(f.logicalCalls, hexAPDU)
	if len(f.logicalResponses) > 0 {
		resp := f.logicalResponses[0]
		f.logicalResponses = f.logicalResponses[1:]
		return resp, nil
	}
	return "9000", nil
}

func TestATAKAProviderUSIMUsesLogicalChannel(t *testing.T) {
	modem := &akaProviderModemFake{
		resolvedAID:   "A0000000871002FF44FF128900000100",
		resolveSource: "qmi_card_status",
		logicalResponses: []string{
			"DB02112210000102030405060708090A0B0C0D0E0F1000101112131415161718191A1B1C1D1E1F9000",
		},
	}
	provider := NewATAKAProvider(modem)

	res, err := provider.CalculateAKAWithPreference(bytes16(0x10), bytes16(0x20), AKAAppPreferenceUSIM)
	if err != nil {
		t.Fatalf("CalculateAKAWithPreference() error = %v", err)
	}

	if len(res.RES) == 0 || len(res.CK) == 0 || len(res.IK) == 0 {
		t.Fatalf("AKA result missing fields: %+v", res)
	}
	if !reflect.DeepEqual(modem.logicalAIDs, []string{"A0000000871002FF44FF128900000100"}) {
		t.Fatalf("logical AIDs = %#v, want resolved full USIM AID", modem.logicalAIDs)
	}
	if len(modem.logicalCalls) == 0 {
		t.Fatal("expected AKA APDU over logical channel")
	}
	if !reflect.DeepEqual(modem.logicalClosed, []int{1}) {
		t.Fatalf("logicalClosed = %#v, want channel 1 closed", modem.logicalClosed)
	}
}

func TestATAKAProviderUSIMUsesResolvedFullAID(t *testing.T) {
	modem := &akaProviderModemFake{
		resolvedAID:   "A0000000871002FF49FF0189",
		resolveSource: "qmi_card_status",
		logicalResponses: []string{
			"DB02112210000102030405060708090A0B0C0D0E0F1000101112131415161718191A1B1C1D1E1F9000",
		},
	}
	provider := NewATAKAProvider(modem)

	_, err := provider.CalculateAKAWithPreference(bytes16(0x10), bytes16(0x20), AKAAppPreferenceUSIM)
	if err != nil {
		t.Fatalf("CalculateAKAWithPreference() error = %v", err)
	}

	if !reflect.DeepEqual(modem.logicalAIDs, []string{"A0000000871002FF49FF0189"}) {
		t.Fatalf("logical AIDs = %#v, want resolved full USIM AID", modem.logicalAIDs)
	}
}

// 打开失败时可以清理残留通道后用同一 AID 重试，但绝不能回退到静态前缀 AID——
// 部分匹配可能选错应用。
func TestATAKAProviderUSIMOpenFailureDoesNotTryStaticFallbackAID(t *testing.T) {
	const fullAID = "A0000000871002FF44FF128900000100"
	modem := &akaProviderModemFake{
		resolvedAID:   fullAID,
		resolveSource: "qmi_card_status",
		openErrByAID: map[string]error{
			fullAID: errors.New("current aid temporarily rejected"),
		},
	}
	provider := NewATAKAProvider(modem)

	_, err := provider.CalculateAKAWithPreference(bytes16(0x10), bytes16(0x20), AKAAppPreferenceUSIM)
	if err == nil {
		t.Fatal("CalculateAKAWithPreference() err=nil, want open failure")
	}

	for _, aid := range modem.logicalAIDs {
		if aid != fullAID {
			t.Fatalf("尝试了非 full AID 的候选 %q，静态前缀回退是被禁止的", aid)
		}
	}
	// 清理残留通道属于自愈动作，允许出现在 close 记录里；
	// 但清理范围之外不该有别的 close。
	if !reflect.DeepEqual(modem.logicalClosed, akaCleanupChannelCandidates) {
		t.Fatalf("logicalClosed = %#v, want 清理候选 %#v", modem.logicalClosed, akaCleanupChannelCandidates)
	}
}

// 通道耗尽自愈：首次打开失败 → 盲关残留通道 → 同一 AID 重试成功。
// 这是 EC20 上 QMI_ERR_INTERNAL 死循环的解法，不清理只能拔插模组。
func TestATAKAProviderUSIMOpenRetriesAfterStaleChannelCleanup(t *testing.T) {
	const fullAID = "A0000000871002FF44FF128900000100"
	modem := &akaProviderModemFake{
		resolvedAID:   fullAID,
		resolveSource: "qmi_card_status",
		openErrOnceByAID: map[string]error{
			fullAID: errors.New("QMI error: service=0x0b msg=0x0042 result=0x0001 error=0x0003"),
		},
		logicalResponses: []string{
			"DB02112210000102030405060708090A0B0C0D0E0F1000101112131415161718191A1B1C1D1E1F9000",
		},
	}
	provider := NewATAKAProvider(modem)

	res, err := provider.CalculateAKAWithPreference(bytes16(0x10), bytes16(0x20), AKAAppPreferenceUSIM)
	if err != nil {
		t.Fatalf("清理残留后重试应成功: %v", err)
	}
	if len(res.RES) == 0 {
		t.Fatalf("AKA result missing fields: %+v", res)
	}
	if !reflect.DeepEqual(modem.logicalAIDs, []string{fullAID, fullAID}) {
		t.Fatalf("logical AIDs = %#v, want 同一 AID 重试两次", modem.logicalAIDs)
	}
	// close 序列 = 清理 1~4 + 用完后正常关闭本次打开的通道 1
	want := append(append([]int{}, akaCleanupChannelCandidates...), 1)
	if !reflect.DeepEqual(modem.logicalClosed, want) {
		t.Fatalf("logicalClosed = %#v, want %#v", modem.logicalClosed, want)
	}
}

// ISIM 路径与 USIM 走同一个打开入口，同样具备清理重试自愈。
func TestATAKAProviderISIMOpenRetriesAfterStaleChannelCleanup(t *testing.T) {
	const isimAID = "A0000000871004FFFFFFFF8903020000"
	modem := &akaProviderModemFake{
		resolvedAIDByApp: map[string]string{"isim": isimAID},
		resolveSource:    "qmi_card_status",
		openErrOnceByAID: map[string]error{
			isimAID: errors.New("QMI error: service=0x0b msg=0x0042 result=0x0001 error=0x0003"),
		},
		logicalResponses: []string{
			"DB02112210000102030405060708090A0B0C0D0E0F1000101112131415161718191A1B1C1D1E1F9000",
		},
	}
	provider := NewATAKAProvider(modem)

	if _, err := provider.CalculateAKAWithPreference(bytes16(0x10), bytes16(0x20), "isim_strict"); err != nil {
		t.Fatalf("ISIM 清理残留后重试应成功: %v", err)
	}
	if !reflect.DeepEqual(modem.logicalAIDs, []string{isimAID, isimAID}) {
		t.Fatalf("logical AIDs = %#v, want 同一 ISIM AID 重试两次", modem.logicalAIDs)
	}
}

func TestATAKAProviderISIMStrictDoesNotFallbackToUSIM(t *testing.T) {
	modem := &akaProviderModemFake{
		resolvedAID:    "A0000000871004FFFFFFFF8903020000",
		resolveSource:  "qmi_card_status",
		openLogicalErr: errors.New("isim open failed"),
	}
	provider := NewATAKAProvider(modem)

	_, err := provider.CalculateAKAWithPreference(bytes16(0x10), bytes16(0x20), "isim_strict")
	if err == nil {
		t.Fatal("CalculateAKAWithPreference() err=nil, want strict ISIM failure")
	}

	if len(modem.logicalCalls) != 0 {
		t.Fatalf("logicalCalls = %#v, want no APDU after open failure", modem.logicalCalls)
	}
	// 清理残留后的同 AID 重试是允许的，但绝不能出现 USIM 的 AID。
	for _, aid := range modem.logicalAIDs {
		if aid != "A0000000871004FFFFFFFF8903020000" {
			t.Fatalf("strict ISIM 失败后尝试了其它 AID %q", aid)
		}
	}
}

func TestWrapPreferredAKAProviderReturnsSWUAKAProvider(t *testing.T) {
	stub := &akaWithPreferenceProviderStub{
		result: swusim.AKAResult{
			RES:  []byte{0x01, 0x02},
			CK:   []byte{0x03, 0x04},
			IK:   []byte{0x05, 0x06},
			AUTS: []byte{0x07, 0x08},
		},
	}

	wrapped := WrapPreferredAKAProvider(stub, AKAAppPreferenceISIMStrict)
	var provider swusim.AKAProvider = wrapped

	got, err := provider.CalculateAKA(bytes16(0x10), bytes16(0x20))
	if err != nil {
		t.Fatalf("CalculateAKA() error = %v", err)
	}
	if stub.lastPreferenceUsed != AKAAppPreferenceISIMStrict {
		t.Fatalf("lastPreferenceUsed = %q, want %q", stub.lastPreferenceUsed, AKAAppPreferenceISIMStrict)
	}
	if !reflect.DeepEqual(got, stub.result) {
		t.Fatalf("CalculateAKA() = %+v, want %+v", got, stub.result)
	}
}

func bytes16(start byte) []byte {
	out := make([]byte, 16)
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}
