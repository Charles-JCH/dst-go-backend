package protocol

import "encoding/json"

const (
	// TypePing Agent 心跳，payload 为 Metrics（旧版本 Agent 不带 payload）
	TypePing    = "ping"
	TypeCommand = "command"
	TypeAck     = "ack"
	TypeLog     = "log"
	TypeResult  = "result"
	// TypeUninstall 平台删除服务器时通知 Agent 卸载自己，不回传结果
	TypeUninstall = "uninstall"
	// TypeSysInfo Agent 每次连上平台后上报一次系统配置
	TypeSysInfo = "sysinfo"

	ActionDeploy = "deploy"
	ActionInit   = "init"
	ActionStart  = "start"
	ActionStop   = "stop"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionStatus = "status"
	// ActionConfig 写入存档配置文件，由 Agent 直接写入，不经过 dst 脚本
	ActionConfig = "config"

	// 平台可以写入的配置文件名：Agent 按名称换算成固定路径，平台不能指定路径
	FileClusterINI         = "cluster.ini"
	FileClusterToken       = "cluster_token.txt"
	FileMasterModOverrides = "Master/modoverrides.lua"
	FileCavesModOverrides  = "Caves/modoverrides.lua"
	FileMasterLevelData    = "Master/leveldataoverride.lua"
	FileCavesLevelData     = "Caves/leveldataoverride.lua"
	// FileModsSetup 服务器上所有存档共用的模组下载列表
	FileModsSetup = "dedicated_server_mods_setup.lua"
)

type Envelope struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type CmdPayload struct {
	Action string `json:"action"`
	Slot   int    `json:"slot,omitempty"`
	// ClusterToken init 时使用，Agent 通过环境变量传给 dst 脚本
	ClusterToken string `json:"clusterToken,omitempty"`
	// Files config 时写入的配置文件
	Files []ConfigFile `json:"files,omitempty"`
}

// ConfigFile 一个配置文件；Name 只能是 File 开头的常量之一
type ConfigFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Log struct {
	Content string `json:"content"`
}

type Ack struct {
	IsAccepted bool   `json:"accepted"`
	Reason     string `json:"reason,omitempty"`
}

// SysInfo 服务器系统配置；ram、disk 单位为字节，disk 为游戏所在分区的容量
type SysInfo struct {
	CPU    int    `json:"cpu"`
	RAM    int64  `json:"ram"`
	Disk   int64  `json:"disk"`
	OSName string `json:"osName"`
	Arch   string `json:"arch"`
}

// Metrics 资源使用情况，随心跳上报；ramUsed、diskUsed 单位为字节，netRx、netTx 单位为字节/秒
type Metrics struct {
	CPUPercent float64 `json:"cpuPercent"`
	RAMUsed    int64   `json:"ramUsed"`
	DiskUsed   int64   `json:"diskUsed"`
	Load1      float64 `json:"load1"`
	NetRx      int64   `json:"netRx"`
	NetTx      int64   `json:"netTx"`
}

type Result struct {
	Action    string `json:"action"`
	Slot      int    `json:"slot,omitempty"`
	IsSuccess bool   `json:"success"`
	ExitCode  int    `json:"exitCode"`
	Error     string `json:"error,omitempty"`
}
