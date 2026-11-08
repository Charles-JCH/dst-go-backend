package response

import (
	"game-panel/internal/model/entity"
	"time"
)

type ServerResp struct {
	ID            uint64        `json:"id"`
	Name          string        `json:"name"`
	IP            string        `json:"ip"`
	Port          int           `json:"port"`
	Username      string        `json:"username"`
	CPU           int           `json:"cpu"`
	RAM           int64         `json:"ram"`
	Disk          int64         `json:"disk"`
	OSName        string        `json:"osName"`
	GameEnv       []GameEnvResp `json:"gameEnv"`
	Mods          []string      `json:"mods"`
	IsAgentOnline bool          `json:"agentOnline"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

type ServerListResp struct {
	List     []ServerResp `json:"list"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

func ToServerResp(server *entity.Server, isAgentOnline bool) *ServerResp {
	gameEnv := make([]GameEnvResp, 0, len(server.GameEnv))
	for _, env := range server.GameEnv {
		gameEnv = append(gameEnv, GameEnvResp{Code: env.Code, Name: env.Name})
	}

	mods := server.Mods
	if mods == nil {
		mods = []string{}
	}

	return &ServerResp{
		ID:            server.ID,
		Name:          server.Name,
		IP:            server.IP,
		Port:          server.Port,
		Username:      server.Username,
		CPU:           server.CPU,
		RAM:           server.RAM,
		Disk:          server.Disk,
		OSName:        server.OSName,
		GameEnv:       gameEnv,
		Mods:          mods,
		IsAgentOnline: isAgentOnline,
		CreatedAt:     server.CreatedAt,
		UpdatedAt:     server.UpdatedAt,
	}
}
