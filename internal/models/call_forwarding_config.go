package models

const (
	CallForwardingStatusSubmitted = "submitted"
	CallForwardingStatusFailed    = "failed"
)

type CallForwardingConfig struct {
	ModuleID     string `gorm:"primaryKey" json:"moduleId"`
	ModuleName   string `json:"moduleName"`
	Enabled      bool   `json:"enabled"`
	Number       string `json:"number"`
	DelaySeconds int    `gorm:"default:20" json:"delaySeconds"`
	LastStatus   string `json:"lastStatus"`
	LastError    string `gorm:"type:text" json:"lastError"`
	UpdatedAt    int64  `json:"updatedAt"`
}

func (CallForwardingConfig) TableName() string {
	return "call_forwarding_configs"
}
