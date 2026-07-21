package models

type MessageType string

const (
	MessageTypeIncoming MessageType = "incoming" // 收到
	MessageTypeOutgoing MessageType = "outgoing" // 发送
)

type MessageStatus string

const (
	MessageStatusReceived MessageStatus = "received" // 接收成功
	MessageStatusSending  MessageStatus = "sending"  // 发送中
	MessageStatusSent     MessageStatus = "sent"     // 发送成功
	MessageStatusFailed   MessageStatus = "failed"   // 发送失败
)

type MessageDecodeStatus string

const (
	MessageDecodeStatusFirmware MessageDecodeStatus = "firmware"
	MessageDecodeStatusDecoded  MessageDecodeStatus = "decoded"
	MessageDecodeStatusFailed   MessageDecodeStatus = "failed"
)

// TextMessage 短信记录
type TextMessage struct {
	ID           string              `gorm:"primaryKey" json:"id"`                      // UUID
	ModuleID     string              `gorm:"index;not null;default:''" json:"moduleId"` // 所属短信模块
	From         string              `gorm:"index" json:"from"`                         // 发送方号码
	RawFrom      string              `json:"rawFrom,omitempty"`                         // 固件上报的原始发送方
	To           string              `gorm:"index" json:"to"`                           // 接收方号码
	Content      string              `gorm:"type:text" json:"content"`                  // 展示和通知使用的短信内容
	RawContent   string              `gorm:"type:text" json:"rawContent,omitempty"`     // 固件解码后的原始内容
	ContentHex   string              `gorm:"type:text" json:"contentHex,omitempty"`     // 固件原始内容的十六进制
	PDUHex       string              `gorm:"type:text" json:"pduHex,omitempty"`         // 完整 PDU；长短信每行一个分片
	DecodeStatus MessageDecodeStatus `gorm:"index" json:"decodeStatus,omitempty"`       // firmware、decoded、failed
	DecodeError  string              `gorm:"type:text" json:"decodeError,omitempty"`    // PDU 解码失败原因
	Alphabet     string              `json:"alphabet,omitempty"`                        // gsm7、ucs2、8bit
	DCS          int                 `json:"dcs,omitempty"`                             // PDU Data Coding Scheme
	SegmentCount int                 `json:"segmentCount,omitempty"`                    // 长短信分片数
	Type         MessageType         `gorm:"index" json:"type"`                         // 消息类型：incoming（收到）、outgoing（发送）
	Status       MessageStatus       `gorm:"index" json:"status"`                       // 状态：received、sent、failed
	CreatedAt    int64               `json:"createdAt" gorm:"autoCreateTime:milli"`     // 创建时间
	UpdatedAt    int64               `json:"updatedAt" gorm:"autoUpdateTime:milli"`     // 更新时间
}

// TableName 指定表名
func (TextMessage) TableName() string {
	return "text_messages"
}
