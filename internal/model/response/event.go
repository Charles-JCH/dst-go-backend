package response

// 推送给浏览器的实时事件类型
const (
	EventTask        = "task"         // 任务状态变化，data 为 TaskResp
	EventTaskLog     = "task_log"     // 任务新增日志，data 为 TaskLogResp
	EventAgentStatus = "agent_status" // Agent 上线或下线，data 为 AgentStatusResp
	EventSysInfo     = "sysinfo"      // Agent 上报了服务器系统配置，data 为 SysInfoResp
	EventMetrics     = "metrics"      // Agent 随心跳上报了资源使用情况，data 为 ServerMetricsResp
)

type AgentStatusResp struct {
	ServerID uint64 `json:"serverID"`
	IsOnline bool   `json:"online"`
}

// SysInfoResp 服务器系统配置；ram、disk 单位为字节
type SysInfoResp struct {
	ServerID uint64 `json:"serverID"`
	CPU      int    `json:"cpu"`
	RAM      int64  `json:"ram"`
	Disk     int64  `json:"disk"`
	OSName   string `json:"osName"`
	Arch     string `json:"arch"`
}

// ServerMetricsResp 某台服务器最新的资源使用情况
type ServerMetricsResp struct {
	ServerID uint64 `json:"serverID"`
	MetricsResp
}
