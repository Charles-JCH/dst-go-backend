package entity

import (
	"fmt"
	"time"
)

const (
	TaskTypeAgentInstall = "agent_install"
	TaskTypeGameDeploy   = "game_deploy" // 通过 Agent 部署游戏环境
	TaskTypeGameUpdate   = "game_update" // 通过 Agent 检查并更新游戏服务端
	TaskTypeDstDelete    = "dst_delete"  // 删除存档：通过 Agent 清理存档文件，成功后删除记录
	TaskTypeDstInit      = "dst_init"    // 新增存档第一步：通过 Agent 拉取默认存档
	TaskTypeDstCreate    = "dst_create"  // 新增存档第二步：写入配置文件，成功后保存存档
	TaskTypeDstUpdate    = "dst_update"  // 编辑存档：写入配置文件，成功后更新存档
	TaskTypeDstStart     = "dst_start"   // 启动存档，成功后标记为运行中
	TaskTypeDstStop      = "dst_stop"    // 停止存档，成功后标记为已停止
	TaskTypeDstStatus    = "dst_status"  // 查看存档运行状态，成功后按实际状态更新
	TaskTypeDstWorld     = "dst_world"   // 更新地面和洞穴的世界设置：写入配置文件，成功后保存

	TaskResourceServer = "server"
	TaskResourceDst    = "dst"

	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"

	// Agent 安装任务的执行步骤
	TaskStageQueued        = "queued"
	TaskStageConnecting    = "connecting"
	TaskStageUploading     = "uploading"
	TaskStageInstalling    = "installing"
	TaskStageWaitingOnline = "waiting_online"

	// Agent 命令任务的执行步骤
	TaskStageExecuting = "executing"

	// taskKeyAgentCommand 同一台服务器同时只能有一个 Agent 命令任务（Agent 一次只执行一条命令）
	taskKeyAgentCommand = "agent_command"
)

type Task struct {
	BaseModel
	Name         string  `gorm:"type:varchar(128);not null;comment:任务名称" json:"name"`
	Type         string  `gorm:"type:varchar(64);not null;comment:任务类型" json:"type"`
	Status       string  `gorm:"type:varchar(16);not null;default:pending;comment:任务状态(pending:待执行 running:执行中 succeeded:成功 failed:失败)" json:"status"`
	Stage        string  `gorm:"type:varchar(64);comment:当前执行步骤" json:"stage"`
	ResourceType string  `gorm:"type:varchar(64);index:idx_task_user_resource,priority:2;comment:关联资源类型" json:"resourceType"`
	ResourceID   uint64  `gorm:"type:bigint;not null;default:0;index:idx_task_user_resource,priority:3;comment:关联资源ID" json:"resourceID"`
	UserID       uint64  `gorm:"type:bigint;not null;index:idx_task_user_resource,priority:1;comment:所属用户ID" json:"userID"`
	CommandID    string  `gorm:"type:varchar(36);index;comment:下发给 Agent 的命令ID，SSH 执行的任务为空" json:"-"`
	ActiveKey    *string `gorm:"type:varchar(128);uniqueIndex;comment:进行中任务的唯一键，结束后置空，保证同一资源同类任务只有一个在执行" json:"-"`
	// Params 任务参数，执行成功后的处理和重试使用，如部署的游戏编码
	Params       map[string]string `gorm:"type:json;serializer:json;comment:任务参数" json:"-"`
	ErrorMessage string            `gorm:"type:text;comment:任务失败原因" json:"errorMessage"`
	StartedAt    *time.Time        `gorm:"comment:开始执行时间" json:"startedAt"`
	FinishedAt   *time.Time        `gorm:"comment:执行结束时间" json:"finishedAt"`
}

func (Task) TableName() string {
	return "task"
}

// TaskActiveKey 生成进行中任务的唯一键：同一资源的同类任务同时只能有一个
func TaskActiveKey(taskType, resourceType string, resourceID uint64) *string {
	key := fmt.Sprintf("%s:%s:%d", taskType, resourceType, resourceID)
	return &key
}

// AgentCommandActiveKey Agent 命令任务的唯一键：按服务器区分，与任务类型和关联资源无关
func AgentCommandActiveKey(serverID uint64) *string {
	return TaskActiveKey(taskKeyAgentCommand, TaskResourceServer, serverID)
}
