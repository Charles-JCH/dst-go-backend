package request

type CreateServerReq struct {
	Name     string `json:"name" binding:"required,max=64"`
	IP       string `json:"ip" binding:"required,ip"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"required,max=256"`
}

type UpdateServerReq struct {
	Name     string `json:"name" binding:"required,max=64"`
	IP       string `json:"ip" binding:"required,ip"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"omitempty,max=256"`
}

type ListServerReq struct {
	Page     int `form:"page,default=1" binding:"min=1,max=1000000"`
	PageSize int `form:"pageSize,default=20" binding:"min=1,max=100"`
}
