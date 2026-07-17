package config

type AppConfig struct {
	JWT       JWTConfig         `json:"JWT"`
	Users     map[string]string `json:"Users"`     // 用户名 -> bcrypt加密的密码
	Serial    SerialConfig      `json:"Serial"`    // 串口配置
	Modules   []ModuleConfig    `json:"Modules"`   // 多模块配置
	Scheduler SchedulerConfig   `json:"Scheduler"` // 定时任务配置
	OIDC      *OIDCConfig       `json:"OIDC"`      // OIDC配置（可选）
}

type SchedulerConfig struct {
	TrafficEndpoint string `json:"TrafficEndpoint"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret       string `json:"Secret"`
	ExpiresHours int    `json:"ExpiresHours"`
}

// SerialConfig 串口配置
type SerialConfig struct {
	Port string `json:"Port"` // 串口路径，为空则自动检测
}

// ModuleConfig 单个短信模块配置
type ModuleConfig struct {
	ID       string `json:"ID"`       // 模块唯一ID，用于API路径
	Name     string `json:"Name"`     // 模块显示名称
	Port     string `json:"Port"`     // 串口路径，为空则自动检测
	Disabled bool   `json:"Disabled"` // 是否禁用该模块
}

// OIDCConfig OIDC认证配置
type OIDCConfig struct {
	Enabled      bool   `json:"Enabled"`      // 是否启用OIDC
	Issuer       string `json:"Issuer"`       // OIDC Provider的Issuer URL
	ClientID     string `json:"ClientID"`     // Client ID
	ClientSecret string `json:"ClientSecret"` // Client Secret
	RedirectURL  string `json:"RedirectURL"`  // 回调URL
}
