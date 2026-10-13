package request

type ListTaskReq struct {
	Page         int    `form:"page,default=1" binding:"min=1,max=1000000"`
	PageSize     int    `form:"pageSize,default=20" binding:"min=1,max=100"`
	Type         string `form:"type" binding:"omitempty,max=64"`
	Status       string `form:"status" binding:"omitempty,oneof=pending running succeeded failed"`
	ResourceType string `form:"resourceType" binding:"omitempty,max=64"`
	ResourceID   uint64 `form:"resourceID"`
}
