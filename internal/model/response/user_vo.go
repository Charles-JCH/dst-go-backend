package response

import (
	"game-panel/internal/model/entity"
)

type UserVO struct {
	ID       uint64  `json:"id"`
	Username string  `json:"username"`
	Phone    string  `json:"phone"`
	Email    *string `json:"email"`
	Role     string  `json:"role"`
	Status   int     `json:"status"`
}

func ToUserVO(u *entity.User) *UserVO {
	return &UserVO{
		ID:       u.ID,
		Username: u.Username,
		Phone:    u.Phone,
		Email:    u.Email,
		Role:     u.Role,
		Status:   u.Status,
	}
}
