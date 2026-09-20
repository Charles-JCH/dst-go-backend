package entity

import (
	"time"
)

type BaseModel struct {
	Id        uint64    `gorm:"primarykey;autoIncrement;comment:主键ID" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime;comment:创建时间" json:"createdAt"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;comment:更新时间" json:"updatedAt"`
}
