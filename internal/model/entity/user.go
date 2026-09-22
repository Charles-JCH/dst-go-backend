package entity

// User 用户实体
type User struct {
	BaseModel
	Username     string  `gorm:"type:varchar(64);uniqueIndex;not null;comment:用户名" json:"username"`
	Password     string  `gorm:"type:varchar(128);not null;comment:密码哈希" json:"-"`
	Phone        string  `gorm:"type:varchar(20);uniqueIndex;not null;comment:手机号" json:"phone"`
	Email        *string `gorm:"type:varchar(128);uniqueIndex;comment:邮箱" json:"email"`
	Role         string  `gorm:"type:varchar(20);default:'user';comment:角色(admin/user)" json:"role"`
	Status       int     `gorm:"type:tinyint;default:1;comment:状态(1:启用 0:禁用)" json:"status"`
	TokenVersion uint64  `gorm:"not null;default:1" json:"-"`
}

func (User) TableName() string {
	return "users"
}
