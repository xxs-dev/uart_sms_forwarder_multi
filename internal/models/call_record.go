package models

const (
	CallStateRinging = "ringing"
	CallStateEnded   = "ended"
)

type CallRecord struct {
	ID         string `gorm:"primaryKey" json:"id"`
	ModuleID   string `gorm:"index" json:"moduleId"`
	ModuleName string `json:"moduleName"`
	From       string `gorm:"column:caller;index" json:"from"`
	State      string `gorm:"index" json:"state"`
	StartedAt  int64  `gorm:"index" json:"startedAt"`
	EndedAt    int64  `json:"endedAt"`
}

func (CallRecord) TableName() string {
	return "call_records"
}
