package request

// InitDstReq 新增存档第一步：在服务器上初始化默认存档
type InitDstReq struct {
	ServerID     uint64 `json:"serverID" binding:"required"`
	Slot         int    `json:"slot" binding:"required,min=1,max=5"`
	ClusterToken string `json:"clusterToken" binding:"required,max=255,printascii"`
}

// CreateDstReq 新增存档第二步：服务器、槽位和令牌取自初始化任务。
// 名称、描述、密码会写入游戏配置文件，不允许换行；密码为空表示不设密码
type CreateDstReq struct {
	InitTaskID  uint64   `json:"initTaskID" binding:"required"`
	Name        string   `json:"name" binding:"required,max=64,singleline"`
	Description string   `json:"description" binding:"max=255,singleline"`
	Password    string   `json:"password" binding:"max=64,printascii"`
	MaxPlayers  int      `json:"maxPlayers" binding:"required,min=1,max=64"`
	Mods        []string `json:"mods" binding:"max=200,dive,required,number,max=32"`
}

// UpdateDstReq 指针字段为 nil 表示不修改；password 传空串表示取消密码，clusterToken 不能改为空
type UpdateDstReq struct {
	Name         string    `json:"name" binding:"required,max=64,singleline"`
	Description  *string   `json:"description" binding:"omitempty,max=255,singleline"`
	Password     *string   `json:"password" binding:"omitempty,max=64,printascii"`
	MaxPlayers   int       `json:"maxPlayers" binding:"required,min=1,max=64"`
	ClusterToken *string   `json:"clusterToken" binding:"omitempty,max=255,printascii"`
	Mods         *[]string `json:"mods" binding:"omitempty,max=200,dive,required,number,max=32"`
}

// ListDstReq serverID 为 0 时查询用户所有服务器的存档
type ListDstReq struct {
	ServerID uint64 `form:"serverID"`
	Page     int    `form:"page,default=1" binding:"min=1,max=1000000"`
	PageSize int    `form:"pageSize,default=20" binding:"min=1,max=100"`
}
