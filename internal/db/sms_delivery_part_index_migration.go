package db

import (
	"strings"

	"gorm.io/gorm"
)

const smsDeliveryPartMIDNoIndex = "idx_sms_delivery_part_mid_no"

// MigrateSMSDeliveryPartUniqueIndex 把 (message_id, part_no) 上的旧普通索引换成唯一索引。
//
// UpsertSMSDeliveryPart 拿这两列当 ON CONFLICT 目标，而 SQLite 只认 PK 与 UNIQUE 索引，
// 早先声明的普通索引会让整条 INSERT 失败，上行短信一条也存不下。
//
// 必须在 AutoMigrate 之前调用：AutoMigrate 只按索引名判断存在与否，同名的旧索引还在时
// 它会直接跳过，不会把索引升级成 UNIQUE。这里把旧索引删掉，后续 AutoMigrate 便会按新
// 声明重建。删除前先去重，否则唯一索引建不起来。
func MigrateSMSDeliveryPartUniqueIndex(tx *gorm.DB) error {
	if tx == nil {
		return nil
	}
	if !tx.Migrator().HasTable(&SMSDeliveryPart{}) {
		return nil // 全新库，AutoMigrate 会直接建出唯一索引
	}

	var indexSQL *string
	if err := tx.Raw(
		"SELECT sql FROM sqlite_master WHERE type = 'index' AND name = ?",
		smsDeliveryPartMIDNoIndex,
	).Scan(&indexSQL).Error; err != nil {
		return err
	}
	if indexSQL == nil {
		return nil // 索引不存在，交给 AutoMigrate
	}
	if strings.Contains(strings.ToUpper(*indexSQL), "UNIQUE") {
		return nil // 已经是唯一索引
	}

	// 同一 (message_id, part_no) 只保留最新一行：分片状态本就是覆盖语义。
	if err := tx.Exec(`DELETE FROM sms_delivery_part WHERE id NOT IN (
		SELECT MAX(id) FROM sms_delivery_part GROUP BY message_id, part_no
	)`).Error; err != nil {
		return err
	}
	return tx.Exec("DROP INDEX IF EXISTS " + smsDeliveryPartMIDNoIndex).Error
}
