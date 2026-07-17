package models

type TrafficRecord struct {
	ID               string `gorm:"primaryKey" json:"id"`
	TaskID           string `gorm:"index" json:"taskId"`
	TaskName         string `json:"taskName"`
	RequestID        string `gorm:"uniqueIndex" json:"requestId"`
	ModuleID         string `gorm:"index" json:"moduleId"`
	ModuleName       string `json:"moduleName"`
	TargetKB         int    `json:"targetKB"`
	Success          bool   `gorm:"index" json:"success"`
	HTTPCode         int64  `json:"httpCode"`
	UplinkBytes      int64  `json:"uplinkBytes"`
	DownlinkBytes    int64  `json:"downlinkBytes"`
	TotalBytes       int64  `json:"totalBytes"`
	BodyBytes        int64  `json:"bodyBytes"`
	ConnectionClosed bool   `json:"connectionClosed"`
	Error            string `gorm:"type:text" json:"error"`
	CreatedAt        int64  `gorm:"index;autoCreateTime:milli" json:"createdAt"`
}

func (TrafficRecord) TableName() string {
	return "traffic_records"
}
