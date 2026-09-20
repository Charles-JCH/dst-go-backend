package response

import (
	"game-panel/internal/model/entity"
)

type UserVO struct {
	Id       uint64 `json:"id"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Status   int    `json:"status"`
}

func ToUserVO(u *entity.User) *UserVO {
	return &UserVO{
		Id:       u.Id,
		Username: u.Username,
		Phone:    u.Phone,
		Role:     u.Role,
		Status:   u.Status,
	}
}
