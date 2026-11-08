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
	Arch          string        `json:"arch"`
	GameEnv       []GameEnvResp `json:"gameEnv"`
	Mods          []string      `json:"mods"`
	IsAgentOnline bool          `json:"agentOnline"`
	Metrics       *MetricsResp  `json:"metrics"` // 最近一次资源使用情况，Agent 离线或没有数据时为 null
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

type ServerListResp struct {
	List     []ServerResp `json:"list"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
}

// MetricsResp 资源使用情况；ramUsed、diskUsed 单位为字节，netRx、netTx 单位为字节/秒
type MetricsResp struct {
	Time       time.Time `json:"time"`
	CPUPercent float64   `json:"cpuPercent"`
	RAMUsed    int64     `json:"ramUsed"`
	DiskUsed   int64     `json:"diskUsed"`
	Load1      float64   `json:"load1"`
	NetRx      int64     `json:"netRx"`
	NetTx      int64     `json:"netTx"`
}

type MetricsListResp struct {
	List []MetricsResp `json:"list"`
}

func ToServerResp(server *entity.Server, isAgentOnline bool, metrics *entity.ServerMetrics) *ServerResp {
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
		Arch:          server.Arch,
		GameEnv:       gameEnv,
		Mods:          mods,
		IsAgentOnline: isAgentOnline,
		Metrics:       ToMetricsResp(metrics),
		CreatedAt:     server.CreatedAt,
		UpdatedAt:     server.UpdatedAt,
	}
}

func ToMetricsResp(m *entity.ServerMetrics) *MetricsResp {
	if m == nil {
		return nil
	}
	return &MetricsResp{
		Time:       m.Time,
		CPUPercent: m.CPUPercent,
		RAMUsed:    m.RAMUsed,
		DiskUsed:   m.DiskUsed,
		Load1:      m.Load1,
		NetRx:      m.NetRx,
		NetTx:      m.NetTx,
	}
}
