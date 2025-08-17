package model

// StepType 定义步骤类型的枚举
type StepType string

const (
	Research   StepType = "research"
	Processing StepType = "processing"
)

// Plan 定义计划的结构体
type Plan struct {
	Locale           string `json:"locale" validate:"required"`
	HasEnoughContext bool   `json:"has_enough_context" validate:"required"`
	Thought          string `json:"thought" validate:"required"`
	Title            string `json:"title" validate:"required"`
	Steps            []Step `json:"steps"`
}

// Step 定义单个步骤的结构体
type Step struct {
	NeedWebSearch bool     `json:"need_web_search" validate:"required"`
	Title         string   `json:"title" validate:"required"`
	Description   string   `json:"description" validate:"required"`
	StepType      StepType `json:"step_type" validate:"required"`
	ExecutionRes  *string  `json:"execution_res,omitempty"`
}
