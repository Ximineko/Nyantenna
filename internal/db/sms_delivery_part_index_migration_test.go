package db

import (
	"path/filepath"
	"testing"
	"time"
)

func initTempDB(t *testing.T) {
	t.Helper()
	if err := Init(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
}

func countParts(t *testing.T, messageID string) int64 {
	t.Helper()
	var n int64
	if err := DB.Model(&SMSDeliveryPart{}).Where("message_id = ?", messageID).Count(&n).Error; err != nil {
		t.Fatalf("统计分片失败: %v", err)
	}
	return n
}

// 全新库：ON CONFLICT 的目标必须真的存在，否则整条 INSERT 会被 SQLite 拒绝
// （"ON CONFLICT clause does not match any PRIMARY KEY or UNIQUE constraint"），
// 上行短信一条也存不下。
func TestUpsertSMSDeliveryPartUpsertsOnFreshDB(t *testing.T) {
	initTempDB(t)
	now := time.Now()

	if err := UpsertSMSDeliveryPart("msg-1", 1, "call-a", 7, SMSDeliveryPartStatePending, now); err != nil {
		t.Fatalf("首次写入分片失败: %v", err)
	}
	if err := UpsertSMSDeliveryPart("msg-1", 1, "call-b", 8, SMSDeliveryPartStateAcked, now.Add(time.Second)); err != nil {
		t.Fatalf("重复写入应更新而非报错: %v", err)
	}

	if got := countParts(t, "msg-1"); got != 1 {
		t.Fatalf("分片行数 = %d, want 1（应为覆盖而非新增）", got)
	}
	var part SMSDeliveryPart
	if err := DB.Where("message_id = ? AND part_no = ?", "msg-1", 1).First(&part).Error; err != nil {
		t.Fatalf("读取分片失败: %v", err)
	}
	if part.CallID != "call-b" || part.State != SMSDeliveryPartStateAcked || part.RPMR != 8 {
		t.Fatalf("分片未被更新: %+v", part)
	}
}

// 旧库：索引同名但不是 UNIQUE。AutoMigrate 只按名字判断存在与否，不会自行升级，
// 因此迁移必须先删掉它——并且删之前要把重复行去掉，否则唯一索引建不起来。
func TestMigrateSMSDeliveryPartUniqueIndexUpgradesLegacyIndex(t *testing.T) {
	initTempDB(t)

	// 复刻旧库形态：换回普通索引，并塞入两条同 (message_id, part_no) 的行
	if err := DB.Exec("DROP INDEX IF EXISTS " + smsDeliveryPartMIDNoIndex).Error; err != nil {
		t.Fatalf("删除唯一索引失败: %v", err)
	}
	if err := DB.Exec("CREATE INDEX " + smsDeliveryPartMIDNoIndex +
		" ON sms_delivery_part(message_id, part_no)").Error; err != nil {
		t.Fatalf("建普通索引失败: %v", err)
	}
	now := time.Now()
	for _, callID := range []string{"call-old", "call-new"} {
		if err := DB.Create(&SMSDeliveryPart{
			MessageID: "msg-legacy", PartNo: 1, CallID: callID,
			State: SMSDeliveryPartStatePending, SentAt: now, CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("造重复行失败: %v", err)
		}
	}

	if err := MigrateSMSDeliveryPartUniqueIndex(DB); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if err := DB.AutoMigrate(&SMSDeliveryPart{}); err != nil {
		t.Fatalf("迁移后 AutoMigrate 失败: %v", err)
	}

	if got := countParts(t, "msg-legacy"); got != 1 {
		t.Fatalf("去重后行数 = %d, want 1", got)
	}
	var part SMSDeliveryPart
	if err := DB.Where("message_id = ?", "msg-legacy").First(&part).Error; err != nil {
		t.Fatalf("读取分片失败: %v", err)
	}
	if part.CallID != "call-new" {
		t.Fatalf("去重应保留最新一行，实际保留 %q", part.CallID)
	}

	// 升级后 upsert 必须可用
	if err := UpsertSMSDeliveryPart("msg-legacy", 1, "call-after", 9, SMSDeliveryPartStateAcked, now); err != nil {
		t.Fatalf("升级后 upsert 仍失败: %v", err)
	}
	if got := countParts(t, "msg-legacy"); got != 1 {
		t.Fatalf("upsert 后行数 = %d, want 1", got)
	}
}

// 迁移可重复执行：已是唯一索引时应原样返回，不得误删数据。
func TestMigrateSMSDeliveryPartUniqueIndexIsIdempotent(t *testing.T) {
	initTempDB(t)
	now := time.Now()
	if err := UpsertSMSDeliveryPart("msg-2", 1, "call-a", 1, SMSDeliveryPartStatePending, now); err != nil {
		t.Fatalf("写入分片失败: %v", err)
	}
	for i := range 2 {
		if err := MigrateSMSDeliveryPartUniqueIndex(DB); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", i+1, err)
		}
	}
	if got := countParts(t, "msg-2"); got != 1 {
		t.Fatalf("重复迁移后行数 = %d, want 1", got)
	}
}
