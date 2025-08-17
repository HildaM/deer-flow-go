package model

import (
	"github.com/cloudwego/eino/schema"
)

type State struct {
	// 用户输入的信息
	Messages []*schema.Message `json:"messages,omitempty"`

	// 子图共享变量
	Goto                           string `json:"goto,omitempty"`
	CurrentPlan                    *Plan  `json:"current_plan,omitempty"`
	Locale                         string `json:"locale,omitempty"`
	PlanIterations                 int    `json:"plan_iterations,omitempty"`
	BackgroundInvestigationResults string `json:"background_investigation_results"`
	InterruptFeedback              string `json:"interrupt_feedback,omitempty"`

	// 全局配置变量
	MaxPlanIterations             int  `json:"max_plan_iterations,omitempty"`
	MaxStepNum                    int  `json:"max_step_num,omitempty"`
	AutoAcceptedPlan              bool `json:"auto_accepted_plan"`
	EnableBackgroundInvestigation bool `json:"enable_background_investigation"`
}
