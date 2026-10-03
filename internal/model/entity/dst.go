package entity

type Dst struct {
	BaseModel
	Slot         int      `gorm:"type:int;not null;default:1;uniqueIndex:uk_dst_server_slot;comment:存档槽位" json:"slot"`
	Name         string   `gorm:"type:varchar(64);not null;comment:存档名称" json:"name"`
	Description  string   `gorm:"type:varchar(255);comment:存档描述" json:"description"`
	Password     string   `gorm:"type:varchar(64);comment:存档密码" json:"password"`
	MaxPlayers   int      `gorm:"type:int;not null;default:6;comment:最大人数" json:"maxPlayers"`
	ClusterToken string   `gorm:"type:varchar(255);not null;comment:科雷令牌" json:"clusterToken"`
	Status       int      `gorm:"type:tinyint;not null;default:0;comment:运行状态(0:停止 1:运行)" json:"status"`
	Mods         []string `gorm:"type:json;serializer:json;comment:模组" json:"mods"`
	ServerID     uint64   `gorm:"type:bigint;not null;uniqueIndex:uk_dst_server_slot;comment:所属服务器ID" json:"serverID"`
	UserID       uint64   `gorm:"type:bigint;index;not null;comment:所属用户ID" json:"userID"`
}

func (Dst) TableName() string {
	return "dst"
}
