package response

// 推送给浏览器的实时事件类型
const (
	EventTask        = "task"         // 任务状态变化，data 为 TaskResp
	EventTaskLog     = "task_log"     // 任务新增日志，data 为 TaskLogResp
	EventAgentStatus = "agent_status" // Agent 上线或下线，data 为 AgentStatusResp
)

type AgentStatusResp struct {
	ServerID uint64 `json:"serverID"`
	IsOnline bool   `json:"online"`
}
