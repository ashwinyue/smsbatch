package model

import "time"

// HistoryM represents a history record
type HistoryM struct {
	ID        int64     `gorm:"column:id;type:bigint(20) unsigned;primaryKey;autoIncrement:true" json:"id"`
	UserID    string    `gorm:"column:user_id;type:varchar(100);not null" json:"user_id"`
	Action    string    `gorm:"column:action;type:varchar(100);not null" json:"action"`
	Resource  string    `gorm:"column:resource;type:varchar(100);not null" json:"resource"`
	Details   string    `gorm:"column:details;type:text" json:"details"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;not null;default:current_timestamp()" json:"created_at"`
}

// TableName returns the table name for HistoryM
func (HistoryM) TableName() string {
	return "nw_history"
}
