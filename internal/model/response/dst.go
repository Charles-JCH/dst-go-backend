package response

import (
	"game-panel/internal/model/entity"
	"time"
)

type DstResp struct {
	ID          uint64    `json:"id"`
	ServerID    uint64    `json:"serverID"`
	Slot        int       `json:"slot"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MaxPlayers  int       `json:"maxPlayers"`
	Status      int       `json:"status"`
	Mods        []string  `json:"mods"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DstListResp struct {
	List     []DstResp `json:"list"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

func ToDstResp(dst *entity.Dst) *DstResp {
	mods := dst.Mods
	if mods == nil {
		mods = []string{}
	}

	return &DstResp{
		ID:          dst.ID,
		ServerID:    dst.ServerID,
		Slot:        dst.Slot,
		Name:        dst.Name,
		Description: dst.Description,
		MaxPlayers:  dst.MaxPlayers,
		Status:      dst.Status,
		Mods:        mods,
		CreatedAt:   dst.CreatedAt,
		UpdatedAt:   dst.UpdatedAt,
	}
}
