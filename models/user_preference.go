package models

import "gorm.io/gorm"

type UserPreference struct {
	gorm.Model
	UserID                     uint   `json:"user_id" gorm:"uniqueIndex;not null"`
	NotificationSoundEnabled   *bool  `json:"notification_sound_enabled" gorm:"default:true"`
	NotificationReportEnabled  *bool  `json:"notification_report_enabled" gorm:"default:true"`
	NotificationStatusEnabled  *bool  `json:"notification_status_enabled" gorm:"default:true"`
	NotificationChatEnabled    *bool  `json:"notification_chat_enabled" gorm:"default:true"`
	Theme                      string `json:"theme" gorm:"type:varchar(20);default:'light';not null"`
	DisplayDensity             string `json:"display_density" gorm:"type:varchar(20);default:'comfortable';not null"`
	MapDefaultView             string `json:"map_default_view" gorm:"type:varchar(20);default:'standard';not null"`
	MapShowLabels              *bool  `json:"map_show_labels" gorm:"default:true"`
	ReportDisplayPreference    string `json:"report_display_preference" gorm:"type:varchar(20);default:'comfortable';not null"`
}

func (p *UserPreference) SoundEnabled() bool {
	if p.NotificationSoundEnabled == nil {
		return true
	}
	return *p.NotificationSoundEnabled
}

func (p *UserPreference) ReportEnabled() bool {
	if p.NotificationReportEnabled == nil {
		return true
	}
	return *p.NotificationReportEnabled
}

func (p *UserPreference) StatusEnabled() bool {
	if p.NotificationStatusEnabled == nil {
		return true
	}
	return *p.NotificationStatusEnabled
}

func (p *UserPreference) ChatEnabled() bool {
	if p.NotificationChatEnabled == nil {
		return true
	}
	return *p.NotificationChatEnabled
}

func (p *UserPreference) ShowLabels() bool {
	if p.MapShowLabels == nil {
		return true
	}
	return *p.MapShowLabels
}

func NewDefaultUserPreference(userID uint) UserPreference {
	defaultTrue := true
	return UserPreference{
		UserID:                    userID,
		NotificationSoundEnabled:  &defaultTrue,
		NotificationReportEnabled: &defaultTrue,
		NotificationStatusEnabled: &defaultTrue,
		NotificationChatEnabled:   &defaultTrue,
		Theme:                     "light",
		DisplayDensity:            "comfortable",
		MapDefaultView:            "standard",
		MapShowLabels:             &defaultTrue,
		ReportDisplayPreference:   "comfortable",
	}
}
