package model

import "time"

// InteractionM represents an interaction record
type InteractionM struct {
	ID          int64     `gorm:"column:id;type:bigint(20) unsigned;primaryKey;autoIncrement:true" json:"id"`
	UserID      string    `gorm:"column:user_id;type:varchar(100);not null" json:"user_id"`
	PhoneNumber string    `gorm:"column:phone_number;type:varchar(20);not null" json:"phone_number"`
	Content     string    `gorm:"column:content;type:text" json:"content"`
	Type        string    `gorm:"column:type;type:varchar(50);not null" json:"type"`
	Status      string    `gorm:"column:status;type:varchar(50);not null" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamp;not null;default:current_timestamp()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:timestamp;not null;default:current_timestamp()" json:"updated_at"`
}

// TableName returns the table name for InteractionM
func (InteractionM) TableName() string {
	return "nw_interaction"
}
