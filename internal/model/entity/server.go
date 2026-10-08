package entity

type Server struct {
	BaseModel
	Name     string   `gorm:"type:varchar(64);not null;comment:服务器名称" json:"name"`
	IP       string   `gorm:"type:varchar(64);not null;uniqueIndex:uk_server_user_ip_port,priority:2;comment:服务器IP" json:"ip"`
	Port     int      `gorm:"type:int;not null;default:22;uniqueIndex:uk_server_user_ip_port,priority:3;comment:SSH端口" json:"port"`
	Username string   `gorm:"type:varchar(64);not null;comment:SSH用户名" json:"username"`
	Password string   `gorm:"type:text;not null;comment:SSH密码密文" json:"-"`
	Cpu      int      `gorm:"type:int;not null;default:0;comment:CPU核心数" json:"cpu"`
	Ram      int64    `gorm:"type:bigint;not null;default:0;comment:内存容量(字节)" json:"ram"`
	Disk     int64    `gorm:"type:bigint;not null;default:0;comment:硬盘容量(字节)" json:"disk"`
	OsName   string   `gorm:"type:varchar(128);comment:操作系统" json:"osName"`
	GameEnv  []string `gorm:"type:json;serializer:json;comment:游戏环境" json:"gameEnv"`
	Mods     []string `gorm:"type:json;serializer:json;comment:模组" json:"mods"`
	UserID   uint64   `gorm:"type:bigint;index;not null;uniqueIndex:uk_server_user_ip_port,priority:1;comment:所属用户ID" json:"userID"`
}

func (Server) TableName() string {
	return "server"
}
