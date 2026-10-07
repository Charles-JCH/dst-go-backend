package response

import (
	"game-panel/internal/model/entity"
)

type UserResp struct {
	ID       uint64  `json:"id"`
	Username string  `json:"username"`
	Phone    string  `json:"phone"`
	Email    *string `json:"email"`
	Role     string  `json:"role"`
	Status   int     `json:"status"`
}

func ToUserResp(u *entity.User) *UserResp {
	return &UserResp{
		ID:       u.ID,
		Username: u.Username,
		Phone:    u.Phone,
		Email:    u.Email,
		Role:     u.Role,
		Status:   u.Status,
	}
}
