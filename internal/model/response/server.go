package response

import (
	"game-panel/internal/model/entity"
	"time"
)

type ServerResp struct {
	ID        uint64        `json:"id"`
	Name      string        `json:"name"`
	IP        string        `json:"ip"`
	Port      int           `json:"port"`
	Username  string        `json:"username"`
	Cpu       int           `json:"cpu"`
	Ram       int64         `json:"ram"`
	Disk      int64         `json:"disk"`
	OsName    string        `json:"osName"`
	GameEnv   []GameEnvResp `json:"gameEnv"`
	Mods      []string      `json:"mods"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type ServerListResp struct {
	List     []ServerResp `json:"list"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

func ToServerResp(server *entity.Server) *ServerResp {
	gameEnv := make([]GameEnvResp, 0, len(server.GameEnv))
	for _, env := range server.GameEnv {
		gameEnv = append(gameEnv, GameEnvResp{Code: env.Code, Name: env.Name})
	}

	mods := server.Mods
	if mods == nil {
		mods = []string{}
	}

	return &ServerResp{
		ID:        server.ID,
		Name:      server.Name,
		IP:        server.IP,
		Port:      server.Port,
		Username:  server.Username,
		Cpu:       server.Cpu,
		Ram:       server.Ram,
		Disk:      server.Disk,
		OsName:    server.OsName,
		GameEnv:   gameEnv,
		Mods:      mods,
		CreatedAt: server.CreatedAt,
		UpdatedAt: server.UpdatedAt,
	}
}
