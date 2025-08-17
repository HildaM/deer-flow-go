package conf

// MCPServerConfig MCP服务器配置
type MCPServerConfig struct {
	Command string            `yaml:"command" mapstructure:"command"`             // MCP服务器启动命令
	Args    []string          `yaml:"args" mapstructure:"args"`                   // 命令行参数列表
	Env     map[string]string `yaml:"env,omitempty" mapstructure:"env,omitempty"` // 环境变量映射，可选配置
}

// MCPConfig MCP配置
type MCPConfig struct {
	Servers map[string]MCPServerConfig `yaml:"servers" mapstructure:"servers"` // MCP服务器配置映射，key为服务器名称
}

// Model 单个模型配置
type Model struct {
	ModelID string `yaml:"model_id" mapstructure:"model_id"` // 模型ID
	BaseURL string `yaml:"base_url" mapstructure:"base_url"` // 模型服务的基础URL地址
	APIKey  string `yaml:"api_key" mapstructure:"api_key"`   // 模型服务的API密钥
}

// ModelConfig 模型配置
type ModelConfig struct {
	DefaultModel Model `yaml:"default_model" mapstructure:"default_model"` // 默认使用的模型名称
}

// SettingConfig 应用运行配置
type SettingConfig struct {
	MaxPlanIterations int `yaml:"max_plan_iterations" mapstructure:"max_plan_iterations"` // 最大计划迭代次数
	TotalMaxRound     int `yaml:"total_max_round" mapstructure:"total_max_round"`         // 全局 agent 最大执行轮数
	AgentMaxStep      int `yaml:"agent_max_step" mapstructure:"agent_max_step"`           // 每个 agent 最大执行步骤数
	MaxLimitToken     int `yaml:"max_limit_token" mapstructure:"max_limit_token"`         // 最大限制token数
}

// AppConfig 应用配置
type AppConfig struct {
	MCP     MCPConfig     `yaml:"mcp" mapstructure:"mcp"`         // MCP服务相关配置
	Model   ModelConfig   `yaml:"model" mapstructure:"model"`     // 大语言模型相关配置
	Setting SettingConfig `yaml:"setting" mapstructure:"setting"` // 应用运行时配置参数
}
