package entity

// 存档运行状态
const (
	DstStatusStopped = 0
	DstStatusRunning = 1
)

type Dst struct {
	BaseModel
	Slot         int      `gorm:"type:int;not null;default:1;uniqueIndex:uk_dst_server_slot,priority:2;comment:存档槽位" json:"slot"`
	Name         string   `gorm:"type:varchar(64);not null;comment:存档名称" json:"name"`
	Description  string   `gorm:"type:varchar(255);comment:存档描述" json:"description"`
	Password     string   `gorm:"type:varchar(64);comment:存档密码" json:"-"`
	MaxPlayers   int      `gorm:"type:int;not null;default:6;comment:最大人数" json:"maxPlayers"`
	ClusterToken string   `gorm:"type:text;not null;comment:科雷令牌密文" json:"-"`
	Status       int      `gorm:"type:tinyint;not null;default:0;comment:运行状态(0:停止 1:运行)" json:"status"`
	Mods         []string `gorm:"type:json;serializer:json;comment:模组" json:"mods"`
	// MasterOverrides、CavesOverrides 地面和洞穴的世界设置，只保存与默认值不同的项
	MasterOverrides map[string]string `gorm:"type:json;serializer:json;comment:地面世界设置" json:"masterOverrides"`
	CavesOverrides  map[string]string `gorm:"type:json;serializer:json;comment:洞穴世界设置" json:"cavesOverrides"`
	// IsWorldPending 世界设置已保存但还未应用到服务器
	IsWorldPending bool   `gorm:"not null;default:false;comment:世界设置是否有未应用的修改" json:"isWorldPending"`
	ServerID       uint64 `gorm:"type:bigint;not null;uniqueIndex:uk_dst_server_slot,priority:1;comment:所属服务器ID" json:"serverID"`
	UserID         uint64 `gorm:"type:bigint;index;not null;comment:所属用户ID" json:"userID"`
}

func (Dst) TableName() string {
	return "dst"
}
