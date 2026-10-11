package request

type CreateDstReq struct {
	ServerID     uint64   `json:"serverID" binding:"required"`
	Slot         int      `json:"slot" binding:"required,min=1,max=5"`
	Name         string   `json:"name" binding:"required,max=64"`
	Description  string   `json:"description" binding:"max=255"`
	Password     string   `json:"password" binding:"max=64"`
	MaxPlayers   int      `json:"maxPlayers" binding:"required,min=1"`
	ClusterToken string   `json:"clusterToken" binding:"max=255"`
	Mods         []string `json:"mods" binding:"max=200,dive,required,numeric,max=32"`
}

type UpdateDstReq struct {
	Name         string   `json:"name" binding:"required,max=64"`
	Description  string   `json:"description" binding:"omitempty,max=255"`
	Password     *string  `json:"password" binding:"max=64"`
	MaxPlayers   int      `json:"maxPlayers" binding:"required,min=1"`
	ClusterToken string   `json:"clusterToken" binding:"max=255"`
	Mods         []string `json:"mods" binding:"max=200,dive,required,numeric,max=32"`
}

type ListDstReq struct {
	ServerID uint64 `form:"serverID" binding:"required"`
	Page     int    `form:"page,default=1" binding:"min=1,max=1000000"`
	PageSize int    `form:"pageSize,default=20" binding:"min=1,max=100"`
}
