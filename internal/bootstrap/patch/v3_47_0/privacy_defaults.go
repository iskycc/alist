package v3_47_0

import (
	"encoding/json"

	"github.com/alist-org/alist/v3/internal/conf"
	internaldb "github.com/alist-org/alist/v3/internal/db"
	"github.com/alist-org/alist/v3/internal/model"
	"github.com/alist-org/alist/v3/pkg/utils"
)

const (
	markerKey             = "privacy_defaults_v3_47_0_migrated"
	legacyOcrAPI          = "https://api.alistgo.com/ocr/file/json"
	legacyAliOpenTokenURL = "https://api.alistgo.com/alist/ali_open/token"
)

// ClearHostedPrivacyDefaults removes previously saved hosted defaults that send
// captcha images or refresh tokens to alistgo.com unless users reconfigure them.
func ClearHostedPrivacyDefaults() {
	if migrationDone() {
		return
	}

	rawDB := internaldb.GetDb()

	if err := rawDB.Model(&model.SettingItem{}).
		Where(&model.SettingItem{Key: conf.OcrApi, Value: legacyOcrAPI}).
		Update("value", "").Error; err != nil {
		utils.Log.Errorf("[privacy defaults] failed to clear OCR API default: %v", err)
	}

	storages, _, err := internaldb.GetStorages(1, model.MaxInt)
	if err != nil {
		utils.Log.Errorf("[privacy defaults] failed to list storages: %v", err)
		return
	}

	var updated int
	for i := range storages {
		storage := &storages[i]
		if storage.Driver != "AliyundriveOpen" || storage.Addition == "" {
			continue
		}
		var addition map[string]interface{}
		if err = json.Unmarshal([]byte(storage.Addition), &addition); err != nil {
			utils.Log.Warnf("[privacy defaults] skip invalid storage addition for %s: %v", storage.MountPath, err)
			continue
		}
		if addition["oauth_token_url"] != legacyAliOpenTokenURL {
			continue
		}
		addition["oauth_token_url"] = ""
		raw, err := json.Marshal(addition)
		if err != nil {
			utils.Log.Warnf("[privacy defaults] skip storage %s marshal: %v", storage.MountPath, err)
			continue
		}
		storage.Addition = string(raw)
		if err = internaldb.UpdateStorage(storage); err != nil {
			utils.Log.Errorf("[privacy defaults] failed to update storage %s: %v", storage.MountPath, err)
			continue
		}
		updated++
	}
	if updated > 0 {
		utils.Log.Infof("[privacy defaults] cleared hosted aliyundrive open token URL from %d storages", updated)
	}

	markMigrationDone()
}

func migrationDone() bool {
	var item model.SettingItem
	err := internaldb.GetDb().Where(&model.SettingItem{Key: markerKey}).First(&item).Error
	return err == nil
}

func markMigrationDone() {
	item := model.SettingItem{
		Key:   markerKey,
		Value: "true",
		Type:  conf.TypeBool,
		Flag:  model.DEPRECATED,
	}
	if err := internaldb.GetDb().Save(&item).Error; err != nil {
		utils.Log.Warnf("[privacy defaults] failed to write migration marker: %v", err)
	}
}
