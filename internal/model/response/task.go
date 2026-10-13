package response

import (
	"game-panel/internal/model/entity"
	"time"
)

type TaskResp struct {
	ID           uint64     `json:"id"`
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	Stage        string     `json:"stage"`
	ResourceType string     `json:"resourceType"`
	ResourceID   uint64     `json:"resourceID"`
	ErrorMessage string     `json:"errorMessage"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type TaskListResp struct {
	List     []TaskResp `json:"list"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

func ToTaskResp(task *entity.Task) *TaskResp {
	return &TaskResp{
		ID:           task.ID,
		Name:         task.Name,
		Type:         task.Type,
		Status:       task.Status,
		Stage:        task.Stage,
		ResourceType: task.ResourceType,
		ResourceID:   task.ResourceID,
		ErrorMessage: task.ErrorMessage,
		StartedAt:    task.StartedAt,
		FinishedAt:   task.FinishedAt,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}
