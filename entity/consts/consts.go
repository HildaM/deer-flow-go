package consts

const (
	GraphName = "deer_flow_go_agent" // 代理图名称，用于标识整个工作流
)

// Agent 名字
const (
	Coordinator            = "coordinator"             // 任务协调者，负责整体任务调度和协调
	Planner                = "planner"                 // 计划者，负责制定和优化执行计划
	Reporter               = "reporter"                // 报告者，负责生成和整理报告内容
	Researcher             = "researcher"              // 研究者，负责信息收集和分析
	Coder                  = "coder"                   // 代码生成者，负责编写和优化代码
	ResearchTeam           = "research_team"           // 研究团队，负责协调多个研究任务
	BackgroundInvestigator = "background_investigator" // 背景调查者，负责深度背景信息挖掘
	Human                  = "human_feedback"          // 人工代理，负责人工干预和反馈
)

// GetAgentNameList 返回列表
func GetAgentNameList() []string {
	return []string{
		Coordinator,
		Planner,
		Reporter,
		Researcher,
		Coder,
		ResearchTeam,
		BackgroundInvestigator,
		Human,
	}
}

// 人类选项
const (
	EditPlan   = "edit_plan" // 编辑计划选项，用户选择修改当前计划
	AcceptPlan = "accepted"  // 接受计划选项，用户确认当前计划
)
