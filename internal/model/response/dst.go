package response

import (
	"game-panel/internal/model/entity"
	"time"
)

type DstResp struct {
	ID          uint64 `json:"id"`
	ServerID    uint64 `json:"serverID"`
	Slot        int    `json:"slot"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxPlayers  int    `json:"maxPlayers"`
	// HasPassword 是否设置了房间密码，密码本身不返回
	HasPassword bool     `json:"hasPassword"`
	Status      int      `json:"status"`
	Mods        []string `json:"mods"`
	// MasterOverrides、CavesOverrides 与默认值不同的世界设置，未设置的项取 WorldOptionResp.Default
	MasterOverrides map[string]string `json:"masterOverrides"`
	CavesOverrides  map[string]string `json:"cavesOverrides"`
	// IsWorldPending 世界设置已保存但还未应用到服务器
	IsWorldPending bool      `json:"isWorldPending"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type DstListResp struct {
	List     []DstResp `json:"list"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

// WorldOptionResp 一项世界设置；category 为 worldgen（仅生成新世界时生效）或 settings（重启后生效）
type WorldOptionResp struct {
	Name       string                 `json:"name"`
	Label      string                 `json:"label"`
	Category   string                 `json:"category"`
	Group      string                 `json:"group"`
	GroupLabel string                 `json:"groupLabel"`
	Default    string                 `json:"default"`
	Options    []WorldOptionValueResp `json:"options"`
}

type WorldOptionValueResp struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// WorldOptionsResp 地面和洞穴可修改的世界设置，按游戏中的分组和顺序排列
type WorldOptionsResp struct {
	Master []WorldOptionResp `json:"master"`
	Caves  []WorldOptionResp `json:"caves"`
}

func ToDstResp(dst *entity.Dst) *DstResp {
	mods := dst.Mods
	if mods == nil {
		mods = []string{}
	}

	return &DstResp{
		ID:              dst.ID,
		ServerID:        dst.ServerID,
		Slot:            dst.Slot,
		Name:            dst.Name,
		Description:     dst.Description,
		MaxPlayers:      dst.MaxPlayers,
		HasPassword:     dst.Password != "",
		Status:          dst.Status,
		Mods:            mods,
		MasterOverrides: nonNilMap(dst.MasterOverrides),
		CavesOverrides:  nonNilMap(dst.CavesOverrides),
		IsWorldPending:  dst.IsWorldPending,
		CreatedAt:       dst.CreatedAt,
		UpdatedAt:       dst.UpdatedAt,
	}
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
