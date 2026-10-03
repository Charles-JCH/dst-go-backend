package entity

type GameEnv struct {
	BaseModel
	Code string `gorm:"type:varchar(32);not null;uniqueIndex;comment:游戏编码" json:"code"`
	Name string `gorm:"type:varchar(64);not null;comment:游戏名称" json:"name"`
}

func (GameEnv) TableName() string {
	return "game_env"
}
