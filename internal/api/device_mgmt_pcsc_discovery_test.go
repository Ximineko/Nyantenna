package api

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ximineko/nyantenna/internal/device"
)

// 一台只插 PC/SC 读卡器、没有任何模组的机器上，模组枚举必然失败。
// 该失败不能吞掉读卡器——否则界面只会显示"暂无可添加设备"。
func TestDiscoveredReturnsPCSCWhenModemScanFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	origModem := discoverCompatibleModemsFromQMIFn
	origQMI := discoverQMIForMgmtFn
	origReaders := listPCSCReadersFn
	defer func() {
		discoverCompatibleModemsFromQMIFn = origModem
		discoverQMIForMgmtFn = origQMI
		listPCSCReadersFn = origReaders
	}()

	discoverQMIForMgmtFn = func() ([]device.QMIDevice, error) { return nil, errors.New("未发现调制解调器") }
	discoverCompatibleModemsFromQMIFn = func([]device.QMIDevice) ([]device.CompatibleModem, error) {
		return nil, errors.New("未发现调制解调器")
	}
	listPCSCReadersFn = func() ([]string, error) { return []string{"Alcor Link AK9563 00 00"}, nil }

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/devices/discovered?with_imei=1", nil)

	(&Server{}).handleDeviceMgmtDiscovered(c)

	var resp struct {
		Devices []struct {
			DiscoveryKey string `json:"discovery_key"`
			Mode         string `json:"mode"`
			Configured   bool   `json:"configured"`
		} `json:"devices"`
		PCSCError string `json:"pcsc_error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应解析失败: %v (body=%s)", err, w.Body.String())
	}
	if len(resp.Devices) != 1 {
		t.Fatalf("应返回 1 个读卡器，实际 %d 个: %s", len(resp.Devices), w.Body.String())
	}
	if resp.Devices[0].Mode != "pcsc" || resp.Devices[0].Configured {
		t.Fatalf("读卡器条目异常: %+v", resp.Devices[0])
	}
	if resp.PCSCError != "" {
		t.Fatalf("枚举成功时不应带 pcsc_error: %q", resp.PCSCError)
	}
}
