package entity

import "time"

const (
	TaskTypeAgentDeploy = "agent_deploy"
	TaskResourceServer  = "server"
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"
)

type Task struct {
	BaseModel
	Name         string     `gorm:"type:varchar(128);not null;comment:任务名称" json:"name"`
	Type         string     `gorm:"type:varchar(64);not null;index;comment:任务类型" json:"type"`
	Status       string     `gorm:"type:varchar(16);not null;default:pending;index;comment:任务状态(pending:待执行 running:执行中 succeeded:成功 failed:失败)" json:"status"`
	Stage        string     `gorm:"type:varchar(64);comment:当前执行步骤" json:"stage"`
	ResourceType string     `gorm:"type:varchar(64);index:idx_task_resource,priority:1;comment:关联资源类型" json:"resourceType"`
	ResourceID   uint64     `gorm:"type:bigint;not null;default:0;index:idx_task_resource,priority:2;comment:关联资源ID" json:"resourceID"`
	UserID       uint64     `gorm:"type:bigint;not null;index;comment:所属用户ID" json:"userID"`
	ErrorMessage string     `gorm:"type:text;comment:任务失败原因" json:"errorMessage"`
	StartedAt    *time.Time `gorm:"comment:开始执行时间" json:"startedAt"`
	FinishedAt   *time.Time `gorm:"comment:执行结束时间" json:"finishedAt"`
}

func (Task) TableName() string {
	return "task"
}
