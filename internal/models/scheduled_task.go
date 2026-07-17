package models

type LastRunStatus string

type ScheduledTaskType string

const (
	LastRunStatusUnknown LastRunStatus = "unknown"
	LastRunStatusSuccess LastRunStatus = "success"
	LastRunStatusFailed  LastRunStatus = "failed"

	ScheduledTaskTypeSMS     ScheduledTaskType = "sms"
	ScheduledTaskTypeTraffic ScheduledTaskType = "traffic"
)

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID           string            `gorm:"primaryKey" json:"id"`                  // UUID
	Name         string            `json:"name"`                                  // 任务名称
	Enabled      bool              `json:"enabled"`                               // 是否启用
	IntervalDays int               `json:"intervalDays"`                          // 执行间隔天数，例如 90 表示每90天执行一次
	TaskType     ScheduledTaskType `gorm:"default:sms" json:"taskType"`           // 任务类型：sms 或 traffic
	ModuleID     string            `json:"moduleId"`                              // 执行任务的模块
	PhoneNumber  string            `json:"phoneNumber"`                           // 目标手机号
	Content      string            `gorm:"type:text" json:"content"`              // 短信内容
	TrafficKB    int               `gorm:"default:5" json:"trafficKB"`            // 流量任务的目标流量（KB）
	CreatedAt    int64             `json:"createdAt" gorm:"autoCreateTime:milli"` // 创建时间（时间戳毫秒）
	UpdatedAt    int64             `json:"updatedAt" gorm:"autoUpdateTime:milli"` // 更新时间（时间戳毫秒）

	LastMsgId     string        `json:"lastMsgId"`                      // 上次发送的短信ID
	LastRunAt     int64         `json:"lastRunAt"`                      // 上次执行时间（时间戳毫秒）
	LastRunStatus LastRunStatus `json:"lastRunStatus"`                  // 上次执行状态
	LastRunDetail string        `gorm:"type:text" json:"lastRunDetail"` // 上次执行的流量和错误明细
}

func (ScheduledTask) TableName() string {
	return "scheduled_tasks"
}
