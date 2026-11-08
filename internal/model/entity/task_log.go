package entity

import "time"

// TaskLog 任务执行日志，一行一条，按 ID 递增排序
type TaskLog struct {
	ID        uint64    `gorm:"primarykey;autoIncrement;comment:主键ID" json:"id"`
	TaskID    uint64    `gorm:"type:bigint;not null;index;comment:所属任务ID" json:"taskID"`
	Content   string    `gorm:"type:text;not null;comment:日志内容" json:"content"`
	CreatedAt time.Time `gorm:"autoCreateTime;comment:创建时间" json:"createdAt"`
}

func (TaskLog) TableName() string {
	return "task_log"
}
